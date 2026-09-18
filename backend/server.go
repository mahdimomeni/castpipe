package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Server handles incoming HTTP drops and uploads.
type Server struct {
	listener    net.Listener
	httpServer  *http.Server
	ip          string
	hostname    string
	port        int
	downloadDir string

	mu           sync.RWMutex
	dropCallback func(item DropItem)
}

// ServerConfig holds configuration options for the HTTP receiver.
type ServerConfig struct {
	DownloadDir string
	Port        int
	Hostname    string
}

// NewServer initializes an HTTP receiver bound to an ephemeral port (:0).
func NewServer(customDownloadDir string) (*Server, error) {
	return NewServerWithConfig(ServerConfig{
		DownloadDir: customDownloadDir,
	})
}

// NewServerWithConfig initializes an HTTP receiver with customizable options.
func NewServerWithConfig(cfg ServerConfig) (*Server, error) {
	bindAddr := ":0"
	if cfg.Port > 0 {
		bindAddr = fmt.Sprintf(":%d", cfg.Port)
	}

	ln, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to bind port %s: %w", bindAddr, err)
	}

	tcpAddr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		_ = ln.Close()
		return nil, fmt.Errorf("unexpected listener address type: %T", ln.Addr())
	}
	port := tcpAddr.Port

	hostname := strings.TrimSpace(cfg.Hostname)
	if hostname == "" {
		h, err := os.Hostname()
		if err != nil || h == "" {
			hostname = "localhost"
		} else {
			hostname = h
		}
	}

	ip := DetectLocalIP()

	downloadDir := strings.TrimSpace(cfg.DownloadDir)
	if downloadDir == "" {
		homeDir, err := os.UserHomeDir()
		folderName := "Castpipe"
		// If custom hostname provided, isolate download directory by default
		if strings.TrimSpace(cfg.Hostname) != "" {
			folderName = fmt.Sprintf("Castpipe-%s", strings.TrimSpace(cfg.Hostname))
		}
		if err != nil {
			downloadDir = filepath.Join(".", "Downloads", folderName)
		} else {
			downloadDir = filepath.Join(homeDir, "Downloads", folderName)
		}
	}
	downloadDir = filepath.Clean(downloadDir)

	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("failed to create download directory %s: %w", downloadDir, err)
	}

	s := &Server{
		listener:    ln,
		ip:          ip,
		hostname:    hostname,
		port:        port,
		downloadDir: downloadDir,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/drop", s.handleDrop)
	mux.HandleFunc("/api/upload", s.handleUpload)
	mux.HandleFunc("/api/health", s.handleHealth)

	s.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Minute,
		WriteTimeout: 10 * time.Minute,
	}

	return s, nil
}

// Start runs the HTTP listener in a background goroutine.
func (s *Server) Start() {
	go func() {
		if err := s.httpServer.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[Castpipe Server] HTTP serve error: %v\n", err)
		}
	}()
}

// Stop gracefully shuts down the HTTP receiver.
func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// SetDropCallback registers a callback invoked when a snippet or file is received.
func (s *Server) SetDropCallback(cb func(item DropItem)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dropCallback = cb
}

func (s *Server) emitDrop(item DropItem) {
	s.mu.RLock()
	cb := s.dropCallback
	s.mu.RUnlock()

	if cb != nil {
		cb(item)
	}
}

// Port returns the ephemeral port allocated by the OS.
func (s *Server) Port() int {
	return s.port
}

// IP returns the local IPv4 address.
func (s *Server) IP() string {
	return s.ip
}

// Hostname returns the local machine hostname.
func (s *Server) Hostname() string {
	return s.hostname
}

// DownloadDir returns the Castpipe folder path.
func (s *Server) DownloadDir() string {
	return s.downloadDir
}

// handleHealth responds to lightweight reachability checks.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"hostname": s.hostname,
		"ip":       s.ip,
		"port":     s.port,
	})
}

// handleDrop processes inbound code snippets and terminal commands.
func (s *Server) handleDrop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload DropSnippetPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	if payload.ID == "" {
		payload.ID = uuid.NewString()
	}
	if payload.Timestamp <= 0 {
		payload.Timestamp = time.Now().UnixMilli()
	}
	if payload.SenderIP == "" {
		payload.SenderIP = extractIP(r.RemoteAddr)
	}
	if payload.Sender == "" {
		payload.Sender = payload.SenderIP
	}

	item := DropItem{
		ID:        payload.ID,
		Type:      DropTypeSnippet,
		Sender:    payload.Sender,
		SenderIP:  payload.SenderIP,
		Timestamp: payload.Timestamp,
		Snippet:   &payload,
	}

	s.emitDrop(item)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"id":     payload.ID,
	})
}

// handleUpload processes streaming file and directory zip drops into Castpipe folder.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "Invalid multipart stream: "+err.Error(), http.StatusBadRequest)
		return
	}

	sender := r.Header.Get("X-Sender")
	if sender == "" {
		sender = r.URL.Query().Get("sender")
	}
	senderIP := extractIP(r.RemoteAddr)
	if sender == "" {
		sender = senderIP
	}

	var savedItems []DropFilePayload

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Error reading multipart part: "+err.Error(), http.StatusInternalServerError)
			return
		}

		filename := part.FileName()
		if filename == "" {
			_ = part.Close()
			continue
		}

		// Sanitize and avoid path traversal
		cleanName := filepath.Base(filepath.Clean(filepath.FromSlash(filename)))
		if cleanName == "." || cleanName == "" {
			cleanName = fmt.Sprintf("drop-%d.bin", time.Now().UnixMilli())
		}

		destPath := ResolveUniquePath(s.downloadDir, cleanName)
		outFile, err := os.Create(destPath)
		if err != nil {
			_ = part.Close()
			http.Error(w, "Failed to create destination file: "+err.Error(), http.StatusInternalServerError)
			return
		}

		written, err := io.Copy(outFile, part)
		_ = outFile.Close()
		_ = part.Close()

		if err != nil {
			http.Error(w, "Failed writing file data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		isArchive := strings.EqualFold(filepath.Ext(cleanName), ".zip")

		filePayload := DropFilePayload{
			ID:        uuid.NewString(),
			Sender:    sender,
			SenderIP:  senderIP,
			FileName:  filepath.Base(destPath),
			FileSize:  written,
			FilePath:  filepath.Clean(destPath),
			IsArchive: isArchive,
			Timestamp: time.Now().UnixMilli(),
		}

		dropItem := DropItem{
			ID:        filePayload.ID,
			Type:      DropTypeFile,
			Sender:    sender,
			SenderIP:  senderIP,
			Timestamp: filePayload.Timestamp,
			File:      &filePayload,
		}

		s.emitDrop(dropItem)
		savedItems = append(savedItems, filePayload)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "ok",
		"files":  savedItems,
	})
}

// ResolveUniquePath ensures incoming files do not overwrite existing files by incrementing (1), (2), etc.
func ResolveUniquePath(dir, filename string) string {
	target := filepath.Join(dir, filename)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		return target
	}

	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", base, i, ext)
		candidatePath := filepath.Join(dir, candidate)
		if _, err := os.Stat(candidatePath); os.IsNotExist(err) {
			return candidatePath
		}
	}
}

// DetectLocalIP returns the primary LAN IPv4 address.
func DetectLocalIP() string {
	// Attempt outbound UDP route check (does not actually transmit packets)
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		if udpAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
			ip := udpAddr.IP.To4()
			if ip != nil && !ip.IsLoopback() {
				return ip.String()
			}
		}
	}

	// Fallback scan of network interfaces
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}

	var fallbackIP string
	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp == 0) || (iface.Flags&net.FlagLoopback != 0) {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			ip4 := ip.To4()
			if ip4 == nil || ip4.IsLoopback() {
				continue
			}

			// Prioritize private RFC 1918 addresses
			if isPrivateIPv4(ip4) {
				return ip4.String()
			}
			if fallbackIP == "" {
				fallbackIP = ip4.String()
			}
		}
	}

	if fallbackIP != "" {
		return fallbackIP
	}

	return "127.0.0.1"
}

func isPrivateIPv4(ip net.IP) bool {
	if ip[0] == 10 {
		return true
	}
	if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
		return true
	}
	if ip[0] == 192 && ip[1] == 168 {
		return true
	}
	return false
}

func extractIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

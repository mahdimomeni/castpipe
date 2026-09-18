package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LocalPeerRecord represents an on-disk heartbeat file for a running local process.
type LocalPeerRecord struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	LastSeen int64  `json:"lastSeen"`
}

// LocalPeerProvider coordinates local filesystem peer announcements in a shared temp directory.
type LocalPeerProvider struct {
	self     Peer
	dir      string
	filePath string
	mu       sync.Mutex
}

// DefaultLocalPeersDir returns the platform-appropriate temporary directory for peer exchange.
func DefaultLocalPeersDir() string {
	return filepath.Join(os.TempDir(), "castpipe_local_peers")
}

// NewLocalPeerProvider initializes a local provider with the given self peer and target directory.
func NewLocalPeerProvider(self Peer, dir string) *LocalPeerProvider {
	if strings.TrimSpace(dir) == "" {
		dir = DefaultLocalPeersDir()
	}
	dir = filepath.Clean(dir)

	filePath := filepath.Join(dir, fmt.Sprintf("%s.json", self.ID))

	return &LocalPeerProvider{
		self:     self,
		dir:      dir,
		filePath: filePath,
	}
}

// Publish writes the instance's active presence file to the local peers directory.
func (p *LocalPeerProvider) Publish() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := os.MkdirAll(p.dir, 0755); err != nil {
		return fmt.Errorf("failed to create local peers directory %s: %w", p.dir, err)
	}

	record := LocalPeerRecord{
		ID:       p.self.ID,
		Hostname: p.self.Hostname,
		IP:       "127.0.0.1", // Guaranteed loopback reachability on the same host
		Port:     p.self.Port,
		LastSeen: time.Now().UnixMilli(),
	}

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal local peer record: %w", err)
	}

	// Write atomically via temporary file to prevent partial reads by concurrent processes
	tmpFile := fmt.Sprintf("%s.tmp.%d", p.filePath, time.Now().UnixNano())
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write local peer file: %w", err)
	}

	if err := os.Rename(tmpFile, p.filePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to commit local peer file: %w", err)
	}

	return nil
}

// Heartbeat refreshes the timestamp of this instance's on-disk record.
func (p *LocalPeerProvider) Heartbeat() error {
	return p.Publish()
}

// Scan reads the directory, prunes expired peers, and returns all active remote peers.
func (p *LocalPeerProvider) Scan(ttl time.Duration) ([]Peer, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	entries, err := os.ReadDir(p.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read local peers directory %s: %w", p.dir, err)
	}

	now := time.Now().UnixMilli()
	ttlMs := ttl.Milliseconds()
	var peers []Peer

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		fullPath := filepath.Join(p.dir, entry.Name())

		// Skip own record file
		if entry.Name() == filepath.Base(p.filePath) {
			continue
		}

		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		var rec LocalPeerRecord
		if err := json.Unmarshal(data, &rec); err != nil {
			// Malformed or partial file; delete
			_ = os.Remove(fullPath)
			continue
		}

		// Prune if expired
		if now-rec.LastSeen > ttlMs {
			_ = os.Remove(fullPath)
			continue
		}

		// Don't report self even if ID differed but port & loopback matched
		if rec.ID == p.self.ID || (rec.Port == p.self.Port && (rec.IP == "127.0.0.1" || rec.IP == p.self.IP)) {
			continue
		}

		peers = append(peers, Peer{
			ID:       rec.ID,
			Hostname: rec.Hostname,
			IP:       rec.IP,
			Port:     rec.Port,
			IsSelf:   false,
			LastSeen: rec.LastSeen,
		})
	}

	return peers, nil
}

// Stop removes this instance's announcement file from the filesystem.
func (p *LocalPeerProvider) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.filePath != "" {
		_ = os.Remove(p.filePath)
	}
}

// Dir returns the active directory path where peer files reside.
func (p *LocalPeerProvider) Dir() string {
	return p.dir
}

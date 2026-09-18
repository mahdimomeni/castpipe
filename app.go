package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"castpipe/backend"

	"github.com/google/uuid"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct manages application state, services, and exposed Wails bindings.
type App struct {
	ctx        context.Context
	server     *backend.Server
	discovery  *backend.DiscoveryService
	self       backend.Peer
	mu         sync.RWMutex
	isReady    bool
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// and background services (ephemeral HTTP server & mDNS discovery) are initialized.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 1. Initialize HTTP receiver server on ephemeral port :0
	server, err := backend.NewServer("")
	if err != nil {
		wailsRuntime.LogErrorf(ctx, "Failed to initialize HTTP receiver: %v", err)
		return
	}
	a.server = server

	// Setup drop callback to emit events to frontend
	server.SetDropCallback(func(item backend.DropItem) {
		wailsRuntime.EventsEmit(a.ctx, "drop:received", item)
	})

	server.Start()

	// 2. Build local peer profile
	self := backend.Peer{
		ID:       uuid.NewString(),
		Hostname: server.Hostname(),
		IP:       server.IP(),
		Port:     server.Port(),
		IsSelf:   true,
		LastSeen: time.Now(),
	}
	a.self = self

	// 3. Initialize mDNS discovery service
	discovery := backend.NewDiscoveryService(self, func(peers []backend.Peer) {
		wailsRuntime.EventsEmit(a.ctx, "peers:updated", peers)
	})
	a.discovery = discovery

	if err := discovery.Start(); err != nil {
		wailsRuntime.LogErrorf(ctx, "Failed to start mDNS discovery: %v", err)
	}

	a.mu.Lock()
	a.isReady = true
	a.mu.Unlock()

	wailsRuntime.LogInfof(ctx, "Castpipe listening on %s:%d (DevDrop at %s)", self.IP, self.Port, server.DownloadDir())
}

// shutdown cleans up active listeners and discovery announcers.
func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.discovery != nil {
		a.discovery.Stop()
	}

	if a.server != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = a.server.Stop(shutdownCtx)
	}
}

// GetSelf returns the local node's identity and active network coordinates.
func (a *App) GetSelf() backend.Peer {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.self
}

// SendSnippet dispatches a code snippet or terminal command to a selected peer.
func (a *App) SendSnippet(targetIP string, targetPort int, content string, syntax string) error {
	a.mu.RLock()
	senderName := a.self.Hostname
	a.mu.RUnlock()

	if senderName == "" {
		senderName = "Castpipe"
	}

	return backend.SendSnippet(senderName, targetIP, targetPort, content, syntax)
}

// SendPaths streams multiple files or directories to a selected peer.
func (a *App) SendPaths(targetIP string, targetPort int, paths []string) error {
	a.mu.RLock()
	senderName := a.self.Hostname
	a.mu.RUnlock()

	if senderName == "" {
		senderName = "Castpipe"
	}

	return backend.SendPaths(senderName, targetIP, targetPort, paths)
}

// OpenDownloadsFolder opens the DevDrop folder in Windows Explorer.
func (a *App) OpenDownloadsFolder() error {
	a.mu.RLock()
	downloadDir := ""
	if a.server != nil {
		downloadDir = a.server.DownloadDir()
	}
	a.mu.RUnlock()

	if downloadDir == "" {
		return fmt.Errorf("download directory not initialized")
	}

	return backend.OpenFolderInExplorer(downloadDir)
}

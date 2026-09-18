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

// AppConfig defines configurable parameters for local dev instances.
type AppConfig struct {
	Name        string
	Port        int
	DownloadDir string
}

// App struct manages application state, services, and exposed Wails bindings.
type App struct {
	ctx        context.Context
	cfg        AppConfig
	server     *backend.Server
	discovery  *backend.DiscoveryService
	self       backend.Peer
	mu         sync.RWMutex
	isReady    bool
}

// NewApp creates a new App application struct with default configuration.
func NewApp() *App {
	return NewAppWithConfig(AppConfig{})
}

// NewAppWithConfig creates a new App application struct with specified parameters.
func NewAppWithConfig(cfg AppConfig) *App {
	return &App{
		cfg: cfg,
	}
}

// startup is called when the app starts. The context is saved
// and background services (ephemeral HTTP server & mDNS discovery) are initialized.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 1. Initialize HTTP receiver server with instance configuration
	server, err := backend.NewServerWithConfig(backend.ServerConfig{
		DownloadDir: a.cfg.DownloadDir,
		Port:        a.cfg.Port,
		Hostname:    a.cfg.Name,
	})
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
		LastSeen: time.Now().UnixMilli(),
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

	wailsRuntime.LogInfof(ctx, "Castpipe listening on %s:%d (Castpipe at %s)", self.IP, self.Port, server.DownloadDir())
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

// OpenDownloadsFolder opens the Castpipe folder in Windows Explorer.
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

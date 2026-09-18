package backend

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestLocalDualTerminalIntegration(t *testing.T) {
	// 1. Setup isolated directories for peer exchange and downloads
	peersDir, err := os.MkdirTemp("", "castpipe_integration_peers_*")
	if err != nil {
		t.Fatalf("Failed to create peers dir: %v", err)
	}
	defer os.RemoveAll(peersDir)

	downloadDir1, err := os.MkdirTemp("", "castpipe_downloads_alpha_*")
	if err != nil {
		t.Fatalf("Failed to create downloads dir 1: %v", err)
	}
	defer os.RemoveAll(downloadDir1)

	downloadDir2, err := os.MkdirTemp("", "castpipe_downloads_beta_*")
	if err != nil {
		t.Fatalf("Failed to create downloads dir 2: %v", err)
	}
	defer os.RemoveAll(downloadDir2)

	// 2. Initialize Server 1 (Alpha)
	srv1, err := NewServerWithConfig(ServerConfig{
		DownloadDir: downloadDir1,
		Hostname:    "Dev-Alpha",
		Port:        0, // ephemeral
	})
	if err != nil {
		t.Fatalf("Failed to start server 1: %v", err)
	}
	srv1.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv1.Stop(ctx)
	}()

	var receivedDrops1 []DropItem
	var mu1 sync.Mutex
	srv1.SetDropCallback(func(item DropItem) {
		mu1.Lock()
		defer mu1.Unlock()
		receivedDrops1 = append(receivedDrops1, item)
	})

	// 3. Initialize Server 2 (Beta)
	srv2, err := NewServerWithConfig(ServerConfig{
		DownloadDir: downloadDir2,
		Hostname:    "Dev-Beta",
		Port:        0, // ephemeral
	})
	if err != nil {
		t.Fatalf("Failed to start server 2: %v", err)
	}
	srv2.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv2.Stop(ctx)
	}()

	var receivedDrops2 []DropItem
	var mu2 sync.Mutex
	srv2.SetDropCallback(func(item DropItem) {
		mu2.Lock()
		defer mu2.Unlock()
		receivedDrops2 = append(receivedDrops2, item)
	})

	// 4. Initialize Discovery Services
	self1 := Peer{
		ID:       "id-node-alpha",
		Hostname: srv1.Hostname(),
		IP:       "127.0.0.1",
		Port:     srv1.Port(),
		IsSelf:   true,
	}
	self2 := Peer{
		ID:       "id-node-beta",
		Hostname: srv2.Hostname(),
		IP:       "127.0.0.1",
		Port:     srv2.Port(),
		IsSelf:   true,
	}

	var ds1SawBeta, ds2SawAlpha bool
	var muDisc sync.Mutex

	ds1 := NewDiscoveryService(self1, func(peers []Peer) {
		muDisc.Lock()
		defer muDisc.Unlock()
		for _, p := range peers {
			if p.ID == "id-node-beta" {
				ds1SawBeta = true
			}
		}
	})
	ds1.SetLocalPeersDir(peersDir)
	ds1.scanInterval = 100 * time.Millisecond
	ds1.peerTTL = 5 * time.Second

	ds2 := NewDiscoveryService(self2, func(peers []Peer) {
		muDisc.Lock()
		defer muDisc.Unlock()
		for _, p := range peers {
			if p.ID == "id-node-alpha" {
				ds2SawAlpha = true
			}
		}
	})
	ds2.SetLocalPeersDir(peersDir)
	ds2.scanInterval = 100 * time.Millisecond
	ds2.peerTTL = 5 * time.Second

	if err := ds1.Start(); err != nil {
		t.Fatalf("ds1.Start failed: %v", err)
	}
	defer ds1.Stop()

	if err := ds2.Start(); err != nil {
		t.Fatalf("ds2.Start failed: %v", err)
	}
	defer ds2.Stop()

	// Wait for mutual discovery
	start := time.Now()
	for time.Since(start) < 3*time.Second {
		muDisc.Lock()
		bothFound := ds1SawBeta && ds2SawAlpha
		muDisc.Unlock()
		if bothFound {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	muDisc.Lock()
	if !ds1SawBeta || !ds2SawAlpha {
		t.Fatalf("Mutual discovery failed: ds1SawBeta=%v, ds2SawAlpha=%v", ds1SawBeta, ds2SawAlpha)
	}
	muDisc.Unlock()

	// 5. Test Sending a Snippet from Alpha -> Beta
	snippetText := "git status -s\ngit commit -m 'feat: test'"
	err = SendSnippet(srv1.Hostname(), "127.0.0.1", srv2.Port(), snippetText, "bash")
	if err != nil {
		t.Fatalf("SendSnippet failed: %v", err)
	}

	// Verify Beta received snippet
	var receivedSnippet *DropSnippetPayload
	start = time.Now()
	for time.Since(start) < 2*time.Second {
		mu2.Lock()
		for _, item := range receivedDrops2 {
			if item.Type == DropTypeSnippet && item.Snippet != nil {
				receivedSnippet = item.Snippet
				break
			}
		}
		mu2.Unlock()
		if receivedSnippet != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if receivedSnippet == nil {
		t.Fatal("Dev-Beta did not receive snippet within timeout")
	}
	if receivedSnippet.Sender != "Dev-Alpha" {
		t.Errorf("Expected sender Dev-Alpha, got %s", receivedSnippet.Sender)
	}
	if receivedSnippet.Content != snippetText {
		t.Errorf("Snippet content mismatch: expected %s, got %s", snippetText, receivedSnippet.Content)
	}
	if receivedSnippet.Syntax != "bash" {
		t.Errorf("Expected syntax bash, got %s", receivedSnippet.Syntax)
	}

	// 6. Test Streaming File Transfer from Beta -> Alpha
	testFilePath := filepath.Join(downloadDir2, "sample_artifact.json")
	fileContent := `{"status": "delivered", "node": "Dev-Beta"}`
	if err := os.WriteFile(testFilePath, []byte(fileContent), 0644); err != nil {
		t.Fatalf("Failed to write test source file: %v", err)
	}

	err = SendPaths(srv2.Hostname(), "127.0.0.1", srv1.Port(), []string{testFilePath})
	if err != nil {
		t.Fatalf("SendPaths failed: %v", err)
	}

	// Verify Alpha received file
	var receivedFile *DropFilePayload
	start = time.Now()
	for time.Since(start) < 2*time.Second {
		mu1.Lock()
		for _, item := range receivedDrops1 {
			if item.Type == DropTypeFile && item.File != nil {
				receivedFile = item.File
				break
			}
		}
		mu1.Unlock()
		if receivedFile != nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if receivedFile == nil {
		t.Fatal("Dev-Alpha did not receive file drop within timeout")
	}
	if receivedFile.Sender != "Dev-Beta" {
		t.Errorf("Expected file sender Dev-Beta, got %s", receivedFile.Sender)
	}
	if receivedFile.FileName != "sample_artifact.json" {
		t.Errorf("Expected file name sample_artifact.json, got %s", receivedFile.FileName)
	}

	savedBytes, err := os.ReadFile(receivedFile.FilePath)
	if err != nil {
		t.Fatalf("Failed to read saved drop file: %v", err)
	}
	if string(savedBytes) != fileContent {
		t.Errorf("Saved drop content mismatch: expected %s, got %s", fileContent, string(savedBytes))
	}

	// 7. Verify clean unregistration on Stop()
	ds1.Stop()
	ds2.Stop()

	// Both files in peersDir should be cleaned up
	files, err := os.ReadDir(peersDir)
	if err == nil && len(files) != 0 {
		t.Errorf("Expected 0 files in peers directory after stop, found %d", len(files))
	}
}

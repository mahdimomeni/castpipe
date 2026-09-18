package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestLocalPeerProviderPublishAndScan(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "castpipe_test_local_disc_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	self1 := Peer{
		ID:       "node-alpha",
		Hostname: "Terminal-1",
		IP:       "127.0.0.1",
		Port:     51001,
	}
	self2 := Peer{
		ID:       "node-beta",
		Hostname: "Terminal-2",
		IP:       "127.0.0.1",
		Port:     51002,
	}

	p1 := NewLocalPeerProvider(self1, tempDir)
	p2 := NewLocalPeerProvider(self2, tempDir)

	// Publish node 1
	if err := p1.Publish(); err != nil {
		t.Fatalf("p1.Publish failed: %v", err)
	}

	// Publish node 2
	if err := p2.Publish(); err != nil {
		t.Fatalf("p2.Publish failed: %v", err)
	}

	// Verify p1 sees p2
	peers1, err := p1.Scan(10 * time.Second)
	if err != nil {
		t.Fatalf("p1.Scan failed: %v", err)
	}
	if len(peers1) != 1 {
		t.Fatalf("p1 expected 1 remote peer, got %d", len(peers1))
	}
	if peers1[0].ID != "node-beta" || peers1[0].Hostname != "Terminal-2" || peers1[0].Port != 51002 {
		t.Errorf("p1 unexpected peer data: %+v", peers1[0])
	}

	// Verify p2 sees p1
	peers2, err := p2.Scan(10 * time.Second)
	if err != nil {
		t.Fatalf("p2.Scan failed: %v", err)
	}
	if len(peers2) != 1 {
		t.Fatalf("p2 expected 1 remote peer, got %d", len(peers2))
	}
	if peers2[0].ID != "node-alpha" || peers2[0].Hostname != "Terminal-1" || peers2[0].Port != 51001 {
		t.Errorf("p2 unexpected peer data: %+v", peers2[0])
	}
}

func TestLocalPeerProviderPruningAndCleanup(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "castpipe_test_local_prune_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	self1 := Peer{
		ID:       "node-alive",
		Hostname: "Alive-Node",
		IP:       "127.0.0.1",
		Port:     52001,
	}

	p1 := NewLocalPeerProvider(self1, tempDir)
	if err := p1.Publish(); err != nil {
		t.Fatalf("p1.Publish failed: %v", err)
	}

	// Manually write a stale peer file
	staleRecord := LocalPeerRecord{
		ID:       "node-stale",
		Hostname: "Crashed-Node",
		IP:       "127.0.0.1",
		Port:     52002,
		LastSeen: time.Now().Add(-1 * time.Hour).UnixMilli(),
	}
	staleData, _ := json.Marshal(staleRecord)
	stalePath := filepath.Join(tempDir, "node-stale.json")
	if err := os.WriteFile(stalePath, staleData, 0644); err != nil {
		t.Fatalf("Failed to write stale file: %v", err)
	}

	// Scan with 5-second TTL
	peers, err := p1.Scan(5 * time.Second)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(peers) != 0 {
		t.Errorf("Expected 0 peers (stale pruned), got %d", len(peers))
	}

	// Stale file should be deleted from disk
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Error("Expected stale file to be deleted from disk")
	}

	// Test Stop() removes own presence
	p1.Stop()
	ownFile := filepath.Join(tempDir, "node-alive.json")
	if _, err := os.Stat(ownFile); !os.IsNotExist(err) {
		t.Error("Expected own file to be removed after Stop()")
	}
}

func TestTwoDiscoveryServicesMutualDiscovery(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "castpipe_test_ds_mutual_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	self1 := Peer{
		ID:       "proc-1-id",
		Hostname: "Terminal-One",
		IP:       "127.0.0.1",
		Port:     53001,
	}
	self2 := Peer{
		ID:       "proc-2-id",
		Hostname: "Terminal-Two",
		IP:       "127.0.0.1",
		Port:     53002,
	}

	var d1SawD2, d2SawD1 bool
	var mu sync.Mutex

	ds1 := NewDiscoveryService(self1, func(peers []Peer) {
		mu.Lock()
		defer mu.Unlock()
		for _, p := range peers {
			if p.ID == "proc-2-id" {
				d1SawD2 = true
			}
		}
	})
	ds1.SetLocalPeersDir(tempDir)
	ds1.scanInterval = 100 * time.Millisecond
	ds1.peerTTL = 1 * time.Second

	ds2 := NewDiscoveryService(self2, func(peers []Peer) {
		mu.Lock()
		defer mu.Unlock()
		for _, p := range peers {
			if p.ID == "proc-1-id" {
				d2SawD1 = true
			}
		}
	})
	ds2.SetLocalPeersDir(tempDir)
	ds2.scanInterval = 100 * time.Millisecond
	ds2.peerTTL = 1 * time.Second

	if err := ds1.Start(); err != nil {
		t.Fatalf("ds1.Start failed: %v", err)
	}
	defer ds1.Stop()

	if err := ds2.Start(); err != nil {
		t.Fatalf("ds2.Start failed: %v", err)
	}
	defer ds2.Stop()

	// Wait up to 3 seconds for mutual discovery
	start := time.Now()
	for time.Since(start) < 3*time.Second {
		mu.Lock()
		bothFound := d1SawD2 && d2SawD1
		mu.Unlock()
		if bothFound {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	mu.Lock()
	found1 := d1SawD2
	found2 := d2SawD1
	mu.Unlock()

	if !found1 {
		t.Error("ds1 failed to discover ds2")
	}
	if !found2 {
		t.Error("ds2 failed to discover ds1")
	}
}

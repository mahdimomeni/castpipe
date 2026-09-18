package backend

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/mdns"
)

func TestPeerRegistryOperations(t *testing.T) {
	reg := NewPeerRegistry()

	self := Peer{
		ID:       "self-123",
		Hostname: "MyMachine",
		IP:       "192.168.1.50",
		Port:     8080,
		IsSelf:   true,
	}

	remote1 := Peer{
		ID:       "peer-1",
		Hostname: "Alice-Laptop",
		IP:       "192.168.1.51",
		Port:     8081,
		IsSelf:   false,
	}

	remote2 := Peer{
		ID:       "peer-2",
		Hostname: "Bob-Desktop",
		IP:       "192.168.1.52",
		Port:     8082,
		IsSelf:   false,
	}

	// 1. Test Upsert
	if !reg.Upsert(self) {
		t.Error("Expected true when adding new self peer")
	}
	if !reg.Upsert(remote1) {
		t.Error("Expected true when adding remote1")
	}
	if !reg.Upsert(remote2) {
		t.Error("Expected true when adding remote2")
	}

	// Re-adding existing should return false
	if reg.Upsert(remote1) {
		t.Error("Expected false when updating existing peer")
	}

	if reg.Count() != 3 {
		t.Errorf("Expected 3 peers, got %d", reg.Count())
	}

	// 2. Test GetAll ordering (Self must always be first)
	all := reg.GetAll()
	if len(all) != 3 {
		t.Fatalf("Expected 3 peers from GetAll(), got %d", len(all))
	}
	if !all[0].IsSelf {
		t.Errorf("Expected first peer to be IsSelf, got %v", all[0])
	}

	// 3. Test GetRemotePeers
	remotes := reg.GetRemotePeers()
	if len(remotes) != 2 {
		t.Fatalf("Expected 2 remote peers, got %d", len(remotes))
	}
	for _, p := range remotes {
		if p.IsSelf {
			t.Errorf("Found self peer in GetRemotePeers(): %v", p)
		}
	}

	// 4. Test Get
	found, ok := reg.Get("peer-1")
	if !ok || found.Hostname != "Alice-Laptop" {
		t.Errorf("Failed to retrieve peer-1 properly: %v", found)
	}

	// 5. Test Prune
	// Manually set remote2 LastSeen to past
	reg.mu.Lock()
	p2 := reg.peers["peer-2"]
	p2.LastSeen = time.Now().Add(-1 * time.Hour).UnixMilli()
	reg.peers["peer-2"] = p2

	// Also simulate self having old LastSeen
	pSelf := reg.peers["self-123"]
	pSelf.LastSeen = time.Now().Add(-1 * time.Hour).UnixMilli()
	reg.peers["self-123"] = pSelf
	reg.mu.Unlock()

	pruned := reg.Prune(10 * time.Second)
	if !pruned {
		t.Error("Expected prune to return true when removing stale peer")
	}

	// remote2 should be gone
	if _, ok := reg.Get("peer-2"); ok {
		t.Error("Expected peer-2 to be pruned")
	}

	// self should NOT be pruned
	if _, ok := reg.Get("self-123"); !ok {
		t.Error("Self peer should never be pruned")
	}

	// remote1 should NOT be pruned
	if _, ok := reg.Get("peer-1"); !ok {
		t.Error("peer-1 was fresh and should not be pruned")
	}
}

func TestPeerRegistryConcurrency(t *testing.T) {
	reg := NewPeerRegistry()
	self := Peer{
		ID:       "self-000",
		Hostname: "WorkerMain",
		IP:       "127.0.0.1",
		Port:     9000,
		IsSelf:   true,
	}
	reg.Upsert(self)

	var wg sync.WaitGroup
	workers := 10
	iterations := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		workerID := i
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				p := Peer{
					ID:       fmt.Sprintf("peer-w%d-%d", workerID, j%5),
					Hostname: fmt.Sprintf("node-%d", workerID),
					IP:       "192.168.1.100",
					Port:     9000 + workerID,
				}
				reg.Upsert(p)
				_ = reg.GetAll()
				_ = reg.GetRemotePeers()
				if j%10 == 0 {
					_ = reg.Prune(10 * time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()

	// Should not have panicked and self should still be present
	if _, ok := reg.Get("self-000"); !ok {
		t.Error("Self was lost during concurrency test")
	}
}

func TestParseServiceEntry(t *testing.T) {
	entry := &mdns.ServiceEntry{
		Name:   "castpipe-node1._castpipe._tcp.local.",
		Host:   "Desktop-Test.",
		AddrV4: net.ParseIP("192.168.1.88"),
		Port:   42100,
		InfoFields: []string{
			"id=node-custom-uuid",
			"hostname=Desktop-Custom",
		},
	}

	peer := parseServiceEntry(entry)

	if peer.ID != "node-custom-uuid" {
		t.Errorf("Expected ID 'node-custom-uuid', got '%s'", peer.ID)
	}
	if peer.Hostname != "Desktop-Custom" {
		t.Errorf("Expected Hostname 'Desktop-Custom', got '%s'", peer.Hostname)
	}
	if peer.IP != "192.168.1.88" {
		t.Errorf("Expected IP '192.168.1.88', got '%s'", peer.IP)
	}
	if peer.Port != 42100 {
		t.Errorf("Expected Port 42100, got %d", peer.Port)
	}
}

func TestDiscoveryServiceLifecycle(t *testing.T) {
	self := Peer{
		ID:       "test-discovery-self",
		Hostname: "TestHost",
		IP:       "127.0.0.1",
		Port:     54321,
	}

	var updateCallCount int
	var mu sync.Mutex

	ds := NewDiscoveryService(self, func(peers []Peer) {
		mu.Lock()
		updateCallCount++
		mu.Unlock()
	})
	ds.scanInterval = 100 * time.Millisecond
	ds.peerTTL = 300 * time.Millisecond

	err := ds.Start()
	if err != nil {
		t.Fatalf("Failed to start discovery service: %v", err)
	}

	time.Sleep(250 * time.Millisecond)

	ds.Stop()

	mu.Lock()
	calls := updateCallCount
	mu.Unlock()

	if calls == 0 {
		t.Error("Expected at least one update callback call (initial state)")
	}
}

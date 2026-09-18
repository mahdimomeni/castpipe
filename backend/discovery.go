package backend

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/mdns"
)

const (
	MdnsServiceType = "_castpipe._tcp"
	MdnsDomain      = "local"
	DefaultScanTime = 4 * time.Second
	DefaultPeerTTL  = 12 * time.Second
)

// PeerRegistry maintains a thread-safe map of known peers in the local network.
type PeerRegistry struct {
	mu    sync.RWMutex
	peers map[string]Peer
}

// NewPeerRegistry initializes an empty peer registry.
func NewPeerRegistry() *PeerRegistry {
	return &PeerRegistry{
		peers: make(map[string]Peer),
	}
}

// Upsert adds or updates a peer in the registry. Returns true if it's a newly added peer.
func (r *PeerRegistry) Upsert(peer Peer) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := peerKey(peer)
	existing, exists := r.peers[key]
	if !exists {
		peer.LastSeen = time.Now().UnixMilli()
		r.peers[key] = peer
		return true
	}

	// Update existing record
	existing.LastSeen = time.Now().UnixMilli()
	existing.IP = peer.IP
	existing.Port = peer.Port
	existing.Hostname = peer.Hostname
	existing.IsSelf = peer.IsSelf
	r.peers[key] = existing
	return false
}

// Prune removes peers that haven't been seen within ttl duration. Self is never pruned.
// Returns true if any peer was removed.
func (r *PeerRegistry) Prune(ttl time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UnixMilli()
	ttlMs := ttl.Milliseconds()
	changed := false

	for key, peer := range r.peers {
		if peer.IsSelf {
			continue
		}
		if now-peer.LastSeen > ttlMs {
			delete(r.peers, key)
			changed = true
		}
	}

	return changed
}

// GetAll returns a copy of all registered peers sorted by Hostname.
func (r *PeerRegistry) GetAll() []Peer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Peer, 0, len(r.peers))
	for _, p := range r.peers {
		list = append(list, p)
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].IsSelf != list[j].IsSelf {
			return list[i].IsSelf // Self always first
		}
		return list[i].Hostname < list[j].Hostname
	})

	return list
}

// GetRemotePeers returns all peers excluding self.
func (r *PeerRegistry) GetRemotePeers() []Peer {
	all := r.GetAll()
	remotes := make([]Peer, 0, len(all))
	for _, p := range all {
		if !p.IsSelf {
			remotes = append(remotes, p)
		}
	}
	return remotes
}

// Get looks up a peer by ID or IP:Port key.
func (r *PeerRegistry) Get(id string) (Peer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.peers[id]
	if ok {
		return p, true
	}

	// Fallback scan by peer ID field
	for _, peer := range r.peers {
		if peer.ID == id {
			return peer, true
		}
	}

	return Peer{}, false
}

// Count returns the total number of peers.
func (r *PeerRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.peers)
}

func peerKey(p Peer) string {
	if p.ID != "" {
		return p.ID
	}
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

// DiscoveryService handles mDNS advertisement, local process discovery, and periodic scanning.
type DiscoveryService struct {
	self          Peer
	registry      *PeerRegistry
	mdnsServer    *mdns.Server
	localProvider *LocalPeerProvider
	scanInterval  time.Duration
	peerTTL       time.Duration

	ctx        context.Context
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup

	mu       sync.RWMutex
	onUpdate func([]Peer)
}

// NewDiscoveryService creates a new discovery manager for this local instance.
func NewDiscoveryService(self Peer, onUpdate func([]Peer)) *DiscoveryService {
	if self.ID == "" {
		self.ID = uuid.NewString()
	}
	self.IsSelf = true
	self.LastSeen = time.Now().UnixMilli()

	registry := NewPeerRegistry()
	registry.Upsert(self)

	localProvider := NewLocalPeerProvider(self, "")

	return &DiscoveryService{
		self:          self,
		registry:      registry,
		localProvider: localProvider,
		scanInterval:  2 * time.Second,
		peerTTL:       DefaultPeerTTL,
		onUpdate:      onUpdate,
	}
}

// SetLocalPeersDir allows overriding the directory used for local inter-process discovery (e.g. in tests).
func (d *DiscoveryService) SetLocalPeersDir(dir string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.localProvider = NewLocalPeerProvider(d.self, dir)
}

// Self returns the local node's Peer identity.
func (d *DiscoveryService) Self() Peer {
	return d.self
}

// Registry returns the underlying thread-safe registry.
func (d *DiscoveryService) Registry() *PeerRegistry {
	return d.registry
}

// SetUpdateCallback sets or updates the peer change notification callback.
func (d *DiscoveryService) SetUpdateCallback(cb func([]Peer)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onUpdate = cb
}

func (d *DiscoveryService) notifyUpdate() {
	d.mu.RLock()
	cb := d.onUpdate
	d.mu.RUnlock()

	if cb != nil {
		cb(d.registry.GetAll())
	}
}

// Start registers local discovery and mDNS service advertisement, and begins the scanning loop.
func (d *DiscoveryService) Start() error {
	d.ctx, d.cancelFunc = context.WithCancel(context.Background())

	// 1. Publish presence in local inter-process peer registry
	if d.localProvider != nil {
		if err := d.localProvider.Publish(); err != nil {
			fmt.Printf("[Discovery] Warning: local peer publish failed: %v\n", err)
		}
	}

	// 2. Publish mDNS service for LAN subnet discovery
	txtRecords := []string{
		fmt.Sprintf("id=%s", d.self.ID),
		fmt.Sprintf("hostname=%s", d.self.Hostname),
	}

	parsedIP := net.ParseIP(d.self.IP)
	var ips []net.IP
	if parsedIP != nil {
		ips = append(ips, parsedIP)
	}

	instanceName := fmt.Sprintf("castpipe-%s", d.self.ID[:8])
	service, err := mdns.NewMDNSService(
		instanceName,
		MdnsServiceType,
		"",
		d.self.Hostname+".",
		d.self.Port,
		ips,
		txtRecords,
	)
	if err == nil {
		server, err := mdns.NewServer(&mdns.Config{Zone: service})
		if err == nil {
			d.mdnsServer = server
		} else {
			fmt.Printf("[Discovery] Warning: failed to start mDNS server: %v\n", err)
		}
	} else {
		fmt.Printf("[Discovery] Warning: failed to create mDNS service: %v\n", err)
	}

	// Initial notification with self
	d.notifyUpdate()

	// 3. Start periodic scanning loop
	d.wg.Add(1)
	go d.scanLoop()

	return nil
}

// Stop shuts down local presence advertising, mDNS, and background scanning.
func (d *DiscoveryService) Stop() {
	if d.cancelFunc != nil {
		d.cancelFunc()
	}

	if d.localProvider != nil {
		d.localProvider.Stop()
	}

	if d.mdnsServer != nil {
		_ = d.mdnsServer.Shutdown()
	}

	d.wg.Wait()
}

func (d *DiscoveryService) scanLoop() {
	defer d.wg.Done()

	// Execute initial scan immediately
	d.performScan()

	ticker := time.NewTicker(d.scanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.performScan()
		}
	}
}

func (d *DiscoveryService) performScan() {
	var discoveredPeers []Peer
	var mu sync.Mutex
	var scanWg sync.WaitGroup

	// 1. Scan and heartbeat local inter-process provider
	if d.localProvider != nil {
		scanWg.Add(1)
		go func() {
			defer scanWg.Done()
			_ = d.localProvider.Heartbeat()
			localPeers, err := d.localProvider.Scan(d.peerTTL)
			if err == nil && len(localPeers) > 0 {
				mu.Lock()
				discoveredPeers = append(discoveredPeers, localPeers...)
				mu.Unlock()
			}
		}()
	}

	// 2. Scan mDNS concurrently if server active
	if d.mdnsServer != nil {
		scanWg.Add(1)
		go func() {
			defer scanWg.Done()
			entriesCh := make(chan *mdns.ServiceEntry, 64)

			params := mdns.DefaultParams(MdnsServiceType)
			params.Domain = MdnsDomain
			params.Timeout = 1200 * time.Millisecond
			params.DisableIPv6 = true
			params.Entries = entriesCh

			var collectWg sync.WaitGroup
			collectWg.Add(1)

			go func() {
				defer collectWg.Done()
				for entry := range entriesCh {
					peer := parseServiceEntry(entry)
					if peer.IP != "" && peer.Port > 0 {
						mu.Lock()
						discoveredPeers = append(discoveredPeers, peer)
						mu.Unlock()
					}
				}
			}()

			_ = mdns.QueryContext(d.ctx, params)
			close(entriesCh)
			collectWg.Wait()
		}()
	}

	scanWg.Wait()

	changed := false

	// 3. Update discovered peers
	for _, p := range discoveredPeers {
		// Detect if this is self
		if p.ID == d.self.ID || (p.Port == d.self.Port && (p.IP == "127.0.0.1" || p.IP == d.self.IP)) {
			p.IsSelf = true
			p.ID = d.self.ID
		}

		if isNew := d.registry.Upsert(p); isNew {
			changed = true
		}
	}

	// 4. Prune dead peers
	if pruned := d.registry.Prune(d.peerTTL); pruned {
		changed = true
	}

	if changed {
		d.notifyUpdate()
	}
}

// parseServiceEntry extracts structured Peer information from an mDNS service entry.
func parseServiceEntry(entry *mdns.ServiceEntry) Peer {
	var ip string
	if entry.AddrV4 != nil {
		ip = entry.AddrV4.String()
	} else if entry.Addr != nil {
		ip = entry.Addr.String()
	}

	hostname := strings.TrimSuffix(entry.Host, ".")
	id := ""

	// Parse TXT records
	for _, field := range entry.InfoFields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) == 2 {
			k := strings.ToLower(strings.TrimSpace(parts[0]))
			v := strings.TrimSpace(parts[1])
			switch k {
			case "id":
				id = v
			case "hostname":
				if v != "" {
					hostname = v
				}
			}
		}
	}

	if id == "" {
		id = fmt.Sprintf("%s:%d", ip, entry.Port)
	}
	if hostname == "" {
		hostname = ip
	}

	return Peer{
		ID:       id,
		Hostname: hostname,
		IP:       ip,
		Port:     entry.Port,
		LastSeen: time.Now().UnixMilli(),
	}
}

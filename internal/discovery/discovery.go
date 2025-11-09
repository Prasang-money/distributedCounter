package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Prasang-money/distributedCounter/models"
)

const (
	HeartbeatInterval = 5 * time.Second
	Peertimeout       = 15 * time.Second
	DiscoveryPath     = "/discovery"
)

// Discovery manages peer discovery and heartbeats.
type Discovery struct {
	nodeID     string
	peers      map[string]*PeerInfo
	mu         sync.RWMutex
	httpClient *http.Client
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	ch         chan int64
}

// PeerInfo represents information about a peer node.
type PeerInfo struct {
	NodeId   string
	LastSeen time.Time
}

// NewDiscovery creates a new Discovery instance.
func NewDiscovery(nodeId string, ch chan int64) *Discovery {
	ctx, cancel := context.WithCancel(context.Background())
	discovery := &Discovery{
		nodeID: nodeId,
		peers:  make(map[string]*PeerInfo),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		ctx:    ctx,
		cancel: cancel,
		ch:     ch,
	}
	return discovery
}

// Start begins the discovery process with the given bootstrap peers.
func (d *Discovery) Start(bootstrapPeers []string) error {
	for _, peer := range bootstrapPeers {
		d.addPeer(peer)
	}

	if len(bootstrapPeers) > 0 {
		d.broadcastJoin()
	}

	d.wg.Add(1)
	go d.heartbeatLoop()

	d.wg.Add(1)
	go d.cleanupLoop()

	return nil
}

// Stop stops the discovery process.
func (d *Discovery) Stop() {
	d.cancel()
	d.wg.Wait()
}

// addPeer adds a new peer to the discovery list.
func (d *Discovery) addPeer(nodeID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if nodeID != d.nodeID && d.peers[nodeID] == nil {
		d.peers[nodeID] = &PeerInfo{
			NodeId:   nodeID,
			LastSeen: time.Now(),
		}

		log.Printf("[Discovery] Added peer: %s", nodeID)
	}
}

// removePeer removes a peer from the peer list
// func (d *Discovery) removePeer(nodeID string) {
// 	d.mu.Lock()
// 	defer d.mu.Unlock()

// 	if _, exists := d.peers[nodeID]; exists {
// 		delete(d.peers, nodeID)
// 		log.Printf("[Discovery] Removed peer: %s", nodeID)
// 	}
// }

// GetPeers returns the list of current peers.

func (d *Discovery) GetPeers() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	peers := make([]string, 0, len(d.peers))
	for addr := range d.peers {
		peers = append(peers, addr)
	}
	return peers
}

// GetNodeID returns the node ID of this discovery instance.
func (d *Discovery) GetNodeID() string {
	return d.nodeID
}

// broadcastJoin sends a JOIN message to all known peers.
func (d *Discovery) broadcastJoin() {
	peers := d.GetPeers()

	msg := models.DiscoveryMessage{
		Type:      models.JoinMessage,
		NodeID:    d.nodeID,
		Peers:     peers,
		Timestamp: time.Now(),
	}

	for _, peer := range peers {
		go d.sendDiscoveryMessage(peer, msg)
	}

}

func (d *Discovery) heartbeatLoop() {
	defer d.wg.Done()
	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.sendHeartbeats()
		}
	}
}

func (d *Discovery) cleanupLoop() {
	defer d.wg.Done()

	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.removeDeadPeers()
		}
	}
}

func (d *Discovery) removeDeadPeers() {
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	for nodeID, info := range d.peers {
		if now.Sub(info.LastSeen) > Peertimeout {
			delete(d.peers, nodeID)
			log.Printf("[Discovery] Peer %s timed out and was removed", nodeID)
		}
	}

}

// sendHeartbeats sends heartbeat messages to all known peers.
func (d *Discovery) sendHeartbeats() {
	d.mu.RLock()
	peers := make([]string, 0, len(d.peers))

	for peerID := range d.peers {
		peers = append(peers, peerID)
	}
	d.mu.RUnlock()
	msg := models.DiscoveryMessage{
		Type:      models.HeartbeatMessage,
		NodeID:    d.nodeID,
		Timestamp: time.Now(),
	}

	for _, peer := range peers {

		go d.sendDiscoveryMessage(peer, msg)
	}
}

// sendDiscoveryMessage sends a discovery message to a specific peer.
func (d *Discovery) sendDiscoveryMessage(peerID string, msg models.DiscoveryMessage) error {
	url := fmt.Sprintf("http://%s%s", peerID, DiscoveryPath)

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(d.ctx, http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to %s: %w", peerID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("peer %s returned status %d", peerID, resp.StatusCode)
	}

	// If this was a JOIN message, process the peer list response
	if msg.Type == models.JoinMessage && resp.ContentLength > 0 {
		var res models.DiscoveryResponse

		if err := json.NewDecoder(resp.Body).Decode(&res); err == nil {
			d.mu.Lock()
			defer d.mu.Unlock()
			d.ch <- res.Value
			for _, peer := range res.Peers {
				if peer != d.nodeID && d.peers[peer] == nil {
					d.peers[peer] = &PeerInfo{
						NodeId:   peer,
						LastSeen: time.Now(),
					}
					log.Printf("[Discovery] Learned about peer %s during join", peer)
				}
			}

		}
	}

	return nil

}

// HandleDiscoveryMessage processes an incoming discovery message.
func (d *Discovery) HandleDiscoveryMessage(msg models.DiscoveryMessage) []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch msg.Type {

	case models.JoinMessage:
		if msg.NodeID != d.nodeID {
			d.peers[msg.NodeID] = &PeerInfo{
				NodeId:   msg.NodeID,
				LastSeen: time.Now(),
			}
			log.Printf("[Discovery] Node %s joined the cluster", msg.NodeID)
		}

		// Merge any peers the joining node knows about
		for _, peerID := range msg.Peers {
			if peerID != d.nodeID && d.peers[peerID] == nil {
				d.peers[peerID] = &PeerInfo{
					NodeId:   peerID,
					LastSeen: time.Now(),
				}
			}
		}

		return d.getPeerListUnlocked()

	case models.HeartbeatMessage:
		if msg.NodeID != d.nodeID {

			if peer, exist := d.peers[msg.NodeID]; exist {
				peer.LastSeen = time.Now()
			} else {
				d.peers[msg.NodeID] = &PeerInfo{
					NodeId:   msg.NodeID,
					LastSeen: time.Now(),
				}

				log.Printf("[Discovery] Discovered new peer via heartbeat: %s", msg.NodeID)
			}
		}

	case models.PeerListMessage:

		for _, peerID := range msg.Peers {
			if peerID != d.nodeID && d.peers[peerID] == nil {
				d.peers[peerID] = &PeerInfo{
					NodeId:   peerID,
					LastSeen: time.Now(),
				}
				log.Printf("[Discovery] Learned about peer %s from %s", peerID, msg.NodeID)
			}
		}
	}
	return nil
}

// getPeerListUnlocked returns peer list without locking (caller must hold lock)
func (d *Discovery) getPeerListUnlocked() []string {
	peers := make([]string, 0, len(d.peers))
	for nodeID := range d.peers {
		peers = append(peers, nodeID)
	}
	return peers
}

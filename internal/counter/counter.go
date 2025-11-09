package counter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Prasang-money/distributedCounter/internal/discovery"
	"github.com/Prasang-money/distributedCounter/models"
	"github.com/cenkalti/backoff/v4"
	"github.com/google/uuid"
)

const (
	PropagationPath   = "/propagate"
	MaxRetries        = 5
	InitialBackoff    = 100 * time.Millisecond
	MaxBackoff        = 5 * time.Second
	CleanupInterval   = 1 * time.Minute
	IncrementIDExpiry = 10 * time.Minute
)

type Counter struct {
	value     int64
	mu        sync.RWMutex
	discovery *discovery.Discovery
	seenIDs   map[string]time.Time
	idMu      sync.RWMutex
	client    *http.Client
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	ch        chan int64
}

// NewCounter creates a new Counter instance.
func NewCounter(d *discovery.Discovery, ch chan int64) *Counter {
	ctx, cancel := context.WithCancel(context.Background())
	counter := &Counter{
		value:     0,
		seenIDs:   make(map[string]time.Time),
		discovery: d,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		ctx:    ctx,
		cancel: cancel,
		ch:     ch,
	}
	if len(counter.discovery.GetPeers()) > 0 {
		counter.value = <-ch
	}

	counter.wg.Add(1)
	go counter.cleanupSeenIDsLoop()

	return counter
}

func (c *Counter) Stop() {
	c.cancel()
	c.wg.Wait()
}

// cleanupSeenIDsLoop periodically cleans up old seen increment IDs.
func (c *Counter) cleanupSeenIDsLoop() {
	defer c.wg.Done()
	ticker := time.NewTicker(CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			c.cleanupOldSeenIDs()
		}
	}
}

func (c *Counter) cleanupOldSeenIDs() {
	c.idMu.Lock()
	defer c.idMu.Unlock()

	count := 0
	now := time.Now()
	for id, timestamp := range c.seenIDs {
		if now.Sub(timestamp) > IncrementIDExpiry {
			delete(c.seenIDs, id)
			count++
		}
	}

	if count > 0 {
		log.Printf("[Counter] Cleaned up %d old increment IDs", count)
	}
}

func (c *Counter) Increment() (string, int64, error) {

	incrementID := uuid.New().String()

	newValue := c.incrementLocal(incrementID)

	go c.propagateIncrement(incrementID, newValue)
	return incrementID, newValue, nil
}

func (c *Counter) incrementLocal(incrementID string) int64 {

	c.idMu.Lock()
	if _, seen := c.seenIDs[incrementID]; seen {
		c.idMu.Unlock()
		c.mu.RLock()
		defer c.mu.RUnlock()
		return c.value

	}

	c.seenIDs[incrementID] = time.Now()
	c.idMu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	c.value++
	return c.value
}

// propagateIncrement sends the increment to all peers
func (c *Counter) propagateIncrement(incrementID string, newValue int64) {
	peers := c.discovery.GetPeers()
	msg := models.IncrementPropagation{
		IncrementID: incrementID,
		NodeID:      c.discovery.GetNodeID(),
		TimeStamp:   time.Now(),
		NewValue:    newValue,
	}
	for _, peer := range peers {
		go c.sendIncrementToPeer(peer, msg)
	}

}

// sendIncrementToPeer sends an increment to a specific peer with exponential backoff
func (c *Counter) sendIncrementToPeer(peerID string, msg models.IncrementPropagation) {
	expBackoff := backoff.NewExponentialBackOff()
	expBackoff.InitialInterval = InitialBackoff
	expBackoff.MaxInterval = MaxBackoff
	expBackoff.MaxElapsedTime = 30 * time.Second

	operation := func() error {
		select {
		case <-c.ctx.Done():
			return backoff.Permanent(fmt.Errorf("context cancelled"))
		default:
		}

		return c.sendIncrementMessage(peerID, msg)
	}

	err := backoff.Retry(operation, backoff.WithMaxRetries(expBackoff, MaxRetries))
	if err != nil {
		log.Printf("[Counter] Failed to propagate increment %s to %s after retries: %v",
			msg.IncrementID, peerID, err)
	}
}

// sendIncrementMessage sends an increment message to a peer
func (c *Counter) sendIncrementMessage(peerID string, msg models.IncrementPropagation) error {
	url := fmt.Sprintf("http://%s%s", peerID, PropagationPath)

	data, err := json.Marshal(msg)
	if err != nil {
		return backoff.Permanent(fmt.Errorf("failed to marshal message: %w", err))
	}

	req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, url, bytes.NewBuffer(data))
	if err != nil {
		return backoff.Permanent(fmt.Errorf("failed to create request: %w", err))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request to %s: %w", peerID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("peer %s returned status %d", peerID, resp.StatusCode)
	}

	return nil
}
func (c *Counter) GetValue() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value
}

// HandleIncrementMessage processes an incoming increment propagation message
func (c *Counter) HandleIncrementMessage(msg models.IncrementPropagation) error {
	// Check if we've already seen this increment
	if c.hasSeenIncrement(msg.IncrementID) {
		log.Printf("[Counter] Duplicate increment %s from %s, ignoring", msg.IncrementID, msg.NodeID)
		return nil
	}

	// Apply the increment
	c.applyIncrement(msg.IncrementID)
	log.Printf("[Counter] Applied increment %s from %s", msg.IncrementID, msg.NodeID)

	return nil

}

// hasSeenIncrement checks if an increment ID has been seen before
func (c *Counter) hasSeenIncrement(incrementID string) bool {
	c.idMu.RLock()
	defer c.idMu.RUnlock()
	_, seen := c.seenIDs[incrementID]
	return seen
}

// applyIncrement applies an increment with the given ID
func (c *Counter) applyIncrement(incrementID string) int64 {
	// Check and mark as seen atomically
	c.idMu.Lock()
	if _, seen := c.seenIDs[incrementID]; seen {
		c.idMu.Unlock()
		c.mu.RLock()
		defer c.mu.RUnlock()
		return c.value
	}
	c.seenIDs[incrementID] = time.Now()
	c.idMu.Unlock()

	// Increment the counter
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value++
	return c.value
}

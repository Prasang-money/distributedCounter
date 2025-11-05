package models

import "time"

// MessageType represents the type of message being sent between nodes.
type MessageType string

const (
	HeartbeatMessage MessageType = "heartbeat"
	JoinMessage      MessageType = "join"
	PeerListMessage  MessageType = "peer_list"
	IncrementMessage MessageType = "increment"
)

// DiscoveryMessage represents a message used in the discovery protocol.
type DiscoveryMessage struct {
	Type      MessageType `json:"type"`
	NodeID    string      `json:"node_id"`
	Peers     []string    `json:"peers,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

type PeersResponse struct {
	Peers []string `json:"peers"`
}

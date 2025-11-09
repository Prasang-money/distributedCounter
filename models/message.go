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
	Value     int64       `json:"value"`
}

type PeersResponse struct {
	Peers []string `json:"peers"`
}

type IncrementResponse struct {
	IncrementID string `json:"increment_id"`
	NewValue    int64  `json:"new_value"`
}

type CountResponse struct {
	Count  int64  `json:"count"`
	NodeID string `json:"node_id"`
}

type IncrementPropagation struct {
	IncrementID string    `json:"increment_id"`
	NewValue    int64     `json:"new_value"`
	NodeID      string    `json:"node_id"`
	TimeStamp   time.Time `json:"timestamp"`
}

type DiscoveryResponse struct {
	Peers []string `json:"peers"`
	Value int64    `json:"value"`
}

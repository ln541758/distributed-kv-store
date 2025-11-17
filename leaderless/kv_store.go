package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// KVPair represents a key-value pair with version
type KVPair struct {
	Value   string `json:"value"`
	Version int    `json:"version"`
}

// KVStore is an in-memory key-value store
type KVStore struct {
	store          map[string]KVPair
	mu             sync.RWMutex
	versionCounter int
}

// NewKVStore creates a new KVStore
func NewKVStore() *KVStore {
	return &KVStore{
		store:          make(map[string]KVPair),
		versionCounter: 0,
	}
}

// Set stores a key-value pair with optional version
func (kv *KVStore) Set(key, value string, version *int) int {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	var v int
	if version == nil {
		// Leader writes: increment version
		kv.versionCounter++
		v = kv.versionCounter
	} else {
		// Follower replication: use provided version from leader
		v = *version
		if v > kv.versionCounter {
			kv.versionCounter = v
		}
	}

	kv.store[key] = KVPair{
		Value:   value,
		Version: v,
	}

	return v
}

// Get retrieves a key-value pair
func (kv *KVStore) Get(key string) (KVPair, bool) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	pair, exists := kv.store[key]
	return pair, exists
}

// LeaderlessNode represents a node in the leaderless architecture
type LeaderlessNode struct {
	nodeID   string
	kvStore  *KVStore
	peerURLs []string
	w        int // Write quorum
	r        int // Read quorum
}

// NewLeaderlessNode creates a new leaderless node
func NewLeaderlessNode(nodeID string, peerURLs []string, w, r int) *LeaderlessNode {
	return &LeaderlessNode{
		nodeID:   nodeID,
		kvStore:  NewKVStore(),
		peerURLs: peerURLs,
		w:        w,
		r:        r,
	}
}

// Write performs a write operation - this node becomes the write coordinator
func (ln *LeaderlessNode) Write(key, value string) (int, int, error) {
	if key == "" {
		return 400, 0, fmt.Errorf("key cannot be empty")
	}

	// Coordinator writes locally first
	version := ln.kvStore.Set(key, value, nil)
	successfulWrites := 1 // Self

	// Replicate to all peers (W=N configuration)
	for _, peerURL := range ln.peerURLs {
		// Simulate network delay
		time.Sleep(200 * time.Millisecond)

		if err := ln.replicateToPeer(peerURL, key, value, version); err == nil {
			successfulWrites++
		}
	}

	// W=N: All nodes must write successfully
	if successfulWrites >= ln.w {
		return 201, version, nil
	}

	return 500, version, fmt.Errorf("failed to meet write quorum")
}

// replicateToPeer sends replication request to a peer
func (ln *LeaderlessNode) replicateToPeer(peerURL, key, value string, version int) error {
	payload := map[string]interface{}{
		"key":     key,
		"value":   value,
		"version": version,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := http.Post(
		peerURL+"/replicate",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		return fmt.Errorf("replication failed with status %d", resp.StatusCode)
	}

	return nil
}

// Read performs a read operation (R=1: only read from local)
func (ln *LeaderlessNode) Read(key string) (int, string, int, error) {
	pair, exists := ln.kvStore.Get(key)
	if !exists {
		return 404, "", 0, fmt.Errorf("key not found")
	}
	return 200, pair.Value, pair.Version, nil
}

// Replicate handles replication request from another node
func (ln *LeaderlessNode) Replicate(key, value string, version int) int {
	// Simulate write delay
	time.Sleep(100 * time.Millisecond)

	ln.kvStore.Set(key, value, &version)
	return 201
}

// LocalRead performs a local read (for testing inconsistency)
func (ln *LeaderlessNode) LocalRead(key string) (int, string, int, error) {
	pair, exists := ln.kvStore.Get(key)
	if !exists {
		return 404, "", 0, fmt.Errorf("key not found")
	}
	return 200, pair.Value, pair.Version, nil
}

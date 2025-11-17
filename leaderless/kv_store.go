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

// Read performs a read operation with quorum support
func (ln *LeaderlessNode) Read(key string) (int, string, int, error) {
	// R=1: Fast path - only read from local
	if ln.r == 1 {
		pair, exists := ln.kvStore.Get(key)
		if !exists {
			return 404, "", 0, fmt.Errorf("key not found")
		}
		return 200, pair.Value, pair.Version, nil
	}

	// R>1: Read from multiple nodes and return latest version
	type readResult struct {
		pair  KVPair
		found bool
		err   error
	}

	results := make(chan readResult, len(ln.peerURLs)+1)

	// Read from local node
	go func() {
		pair, exists := ln.kvStore.Get(key)
		results <- readResult{pair: pair, found: exists, err: nil}
	}()

	// Read from peer nodes in parallel
	for _, peerURL := range ln.peerURLs {
		peerURL := peerURL // capture for goroutine
		go func() {
			pair, err := ln.readFromPeer(peerURL, key)
			if err != nil {
				results <- readResult{found: false, err: err}
			} else {
				results <- readResult{pair: pair, found: true, err: nil}
			}
		}()
	}

	// Collect R responses
	var validPairs []KVPair
	nodesRead := 0
	totalNodes := len(ln.peerURLs) + 1

	for i := 0; i < totalNodes && nodesRead < ln.r; i++ {
		result := <-results
		if result.found {
			validPairs = append(validPairs, result.pair)
			nodesRead++
		}
	}

	// Check if R requirement is met
	if nodesRead < ln.r {
		return 500, "", 0, fmt.Errorf("failed to meet read quorum: got %d/%d", nodesRead, ln.r)
	}

	// Return the value with highest version (Last-Write-Wins)
	if len(validPairs) == 0 {
		return 404, "", 0, fmt.Errorf("key not found")
	}

	latest := validPairs[0]
	for _, pair := range validPairs[1:] {
		if pair.Version > latest.Version {
			latest = pair
		}
	}

	return 200, latest.Value, latest.Version, nil
}

// readFromPeer reads a key from a peer node
func (ln *LeaderlessNode) readFromPeer(peerURL, key string) (KVPair, error) {
	resp, err := http.Get(peerURL + "/local_read/" + key)
	if err != nil {
		return KVPair{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return KVPair{}, fmt.Errorf("read failed with status %d", resp.StatusCode)
	}

	var result struct {
		Value   string `json:"value"`
		Version int    `json:"version"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return KVPair{}, err
	}

	return KVPair{Value: result.Value, Version: result.Version}, nil
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

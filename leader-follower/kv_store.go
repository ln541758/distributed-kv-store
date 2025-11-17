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
		kv.versionCounter++
		v = kv.versionCounter
	} else {
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

// LeaderNode represents a leader in the Leader-Follower architecture
type LeaderNode struct {
	kvStore      *KVStore
	followerURLs []string
	w            int // Write quorum
	r            int // Read quorum
}

// NewLeaderNode creates a new leader node
func NewLeaderNode(followerURLs []string, w, r int) *LeaderNode {
	return &LeaderNode{
		kvStore:      NewKVStore(),
		followerURLs: followerURLs,
		w:            w,
		r:            r,
	}
}

// Write performs a write operation with replication
func (ln *LeaderNode) Write(key, value string) (int, int, error) {
	if key == "" {
		return 400, 0, fmt.Errorf("key cannot be empty")
	}

	// Leader writes locally first
	version := ln.kvStore.Set(key, value, nil)
	successfulWrites := 1 // Leader itself

	// W=1: Only leader needs to write
	if ln.w == 1 {
		return 201, version, nil
	}

	// Replicate to followers
	for _, followerURL := range ln.followerURLs {
		// Simulate network delay
		time.Sleep(200 * time.Millisecond)

		if err := ln.replicateToFollower(followerURL, key, value, version); err == nil {
			successfulWrites++

			// Early return if W is satisfied
			if successfulWrites >= ln.w {
				return 201, version, nil
			}
		}
	}

	// Check if W requirement is met
	if successfulWrites >= ln.w {
		return 201, version, nil
	}

	return 500, version, fmt.Errorf("failed to meet write quorum")
}

// replicateToFollower sends replication request to a follower
func (ln *LeaderNode) replicateToFollower(followerURL, key, value string, version int) error {
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
		followerURL+"/replicate",
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

// Read performs a read operation
func (ln *LeaderNode) Read(key string) (int, string, int, error) {
	// R=1: Only read from leader
	if ln.r == 1 {
		pair, exists := ln.kvStore.Get(key)
		if !exists {
			return 404, "", 0, fmt.Errorf("key not found")
		}
		return 200, pair.Value, pair.Version, nil
	}

	// R>1: Read from multiple nodes and return latest version
	results := []KVPair{}

	// Read from leader
	if pair, exists := ln.kvStore.Get(key); exists {
		results = append(results, pair)
	}

	// Read from followers
	nodesRead := 1
	for _, followerURL := range ln.followerURLs {
		if nodesRead >= ln.r {
			break
		}

		if pair, err := ln.readFromFollower(followerURL, key); err == nil {
			results = append(results, pair)
			nodesRead++
		}
	}

	// Check if R requirement is met
	if nodesRead < ln.r {
		return 500, "", 0, fmt.Errorf("failed to meet read quorum")
	}

	// Return the latest version
	if len(results) == 0 {
		return 404, "", 0, fmt.Errorf("key not found")
	}

	latest := results[0]
	for _, pair := range results {
		if pair.Version > latest.Version {
			latest = pair
		}
	}

	return 200, latest.Value, latest.Version, nil
}

// readFromFollower reads from a follower node
func (ln *LeaderNode) readFromFollower(followerURL, key string) (KVPair, error) {
	resp, err := http.Get(followerURL + "/local_read/" + key)
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

// LocalRead performs a local read (for testing)
func (ln *LeaderNode) LocalRead(key string) (int, string, int, error) {
	pair, exists := ln.kvStore.Get(key)
	if !exists {
		return 404, "", 0, fmt.Errorf("key not found")
	}
	return 200, pair.Value, pair.Version, nil
}

// FollowerNode represents a follower in the Leader-Follower architecture
type FollowerNode struct {
	kvStore *KVStore
}

// NewFollowerNode creates a new follower node
func NewFollowerNode() *FollowerNode {
	return &FollowerNode{
		kvStore: NewKVStore(),
	}
}

// Replicate handles replication request from leader
func (fn *FollowerNode) Replicate(key, value string, version int) int {
	// Simulate write delay
	time.Sleep(100 * time.Millisecond)

	fn.kvStore.Set(key, value, &version)
	return 201
}

// LocalRead performs a local read
func (fn *FollowerNode) LocalRead(key string) (int, string, int, error) {
	// Simulate read delay
	time.Sleep(50 * time.Millisecond)

	pair, exists := fn.kvStore.Get(key)
	if !exists {
		return 404, "", 0, fmt.Errorf("key not found")
	}
	return 200, pair.Value, pair.Version, nil
}

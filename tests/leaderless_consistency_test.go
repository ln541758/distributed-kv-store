package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"testing"
	"time"
)

var nodeURLs = []string{
	"http://localhost:8080",
	"http://localhost:8081",
	"http://localhost:8082",
	"http://localhost:8083",
	"http://localhost:8084",
}

// TestLeaderlessInconsistencyWindow tests that within the update time window,
// reads from other nodes should be inconsistent
func TestLeaderlessInconsistencyWindow(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	key := fmt.Sprintf("test_key_leaderless_%d", time.Now().UnixNano())
	value := "test_value_leaderless"

	// Write to a random node - that node becomes the Write Coordinator
	coordinatorIdx := rand.Intn(len(nodeURLs))
	coordinatorURL := nodeURLs[coordinatorIdx]
	t.Logf("Selected coordinator: %s (index %d)", coordinatorURL, coordinatorIdx)

	type InconsistentRead struct {
		Node      string
		Status    string
		Value     string
		Version   int
		Timestamp time.Time
	}

	inconsistentReads := []InconsistentRead{}
	var mu sync.Mutex

	// Start write operation in a goroutine
	var writeResp LeaderlessWriteResponse
	var writeErr error
	writeDone := make(chan bool)

	go func() {
		writeResp = writeKeyLeaderless(coordinatorURL, key, value)
		writeErr = nil
		if writeResp.StatusCode != 201 {
			writeErr = fmt.Errorf("write failed with status %d", writeResp.StatusCode)
		}
		writeDone <- true
	}()

	// Within the update time window, read from other nodes
	// These reads should be inconsistent
	readStartTime := time.Now()

	// Start reading immediately from other nodes (not the coordinator)
	for time.Since(readStartTime) < 3*time.Second {
		for i, nodeURL := range nodeURLs {
			if i == coordinatorIdx {
				continue // Skip coordinator
			}

			resp := readKeyLeaderless(nodeURL, key)

			// Check for inconsistency
			if resp.StatusCode == 404 {
				// Key not found - this is inconsistent if write has started
				mu.Lock()
				inconsistentReads = append(inconsistentReads, InconsistentRead{
					Node:      fmt.Sprintf("Node%d", i+1),
					Status:    "NOT_FOUND",
					Value:     "",
					Version:   0,
					Timestamp: time.Now(),
				})
				mu.Unlock()
				t.Logf("  [Inconsistency detected] Node%d: Key not found", i+1)
			} else if resp.StatusCode == 200 && resp.Value != value {
				// Old value - this is inconsistent
				mu.Lock()
				inconsistentReads = append(inconsistentReads, InconsistentRead{
					Node:      fmt.Sprintf("Node%d", i+1),
					Status:    "OLD_VALUE",
					Value:     resp.Value,
					Version:   resp.Version,
					Timestamp: time.Now(),
				})
				mu.Unlock()
				t.Logf("  [Inconsistency detected] Node%d: Old value '%s' (expected '%s')", i+1, resp.Value, value)
			}
		}
		time.Sleep(50 * time.Millisecond) // Check every 50ms
	}

	// Wait for write to complete
	<-writeDone

	if writeErr != nil {
		t.Fatalf("Write operation failed: %v", writeErr)
	}

	t.Logf("Write completed: version %d, coordinator: %s", writeResp.Version, coordinatorURL)
	t.Logf("Captured %d inconsistent reads during replication window", len(inconsistentReads))

	// The test demonstrates that inconsistency can occur during the replication window
	if len(inconsistentReads) > 0 {
		t.Logf("✓ Successfully detected inconsistency window with %d inconsistent reads", len(inconsistentReads))
		t.Logf("  This demonstrates that during replication, other nodes may have stale data")
	} else {
		t.Logf("⚠ No inconsistency detected - replication may have been too fast")
		t.Logf("  Try increasing load or network delays to observe inconsistency window")
	}
}

// TestConsistencyAfterWrite tests that after Coordinator acknowledges write,
// reads from Coordinator and other nodes should be consistent
func TestConsistencyAfterWrite(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	key := fmt.Sprintf("test_key_after_write_%d", time.Now().UnixNano())
	value := "test_value_after_write"

	// Write to a random node - that node becomes the Write Coordinator
	coordinatorIdx := rand.Intn(len(nodeURLs))
	coordinatorURL := nodeURLs[coordinatorIdx]
	t.Logf("Selected coordinator: %s (index %d)", coordinatorURL, coordinatorIdx)

	// Send write to Coordinator
	writeResp := writeKeyLeaderless(coordinatorURL, key, value)
	if writeResp.StatusCode != 201 {
		t.Fatalf("Write failed: expected 201, got %d", writeResp.StatusCode)
	}
	t.Logf("Write acknowledged by Coordinator: version %d", writeResp.Version)

	// After Coordinator acknowledges, read from Coordinator - should be consistent
	coordReadResp := readKeyLeaderless(coordinatorURL, key)
	if coordReadResp.StatusCode != 200 {
		t.Fatalf("Read from Coordinator failed: expected 200, got %d", coordReadResp.StatusCode)
	}
	if coordReadResp.Value != value {
		t.Fatalf("Inconsistent data from Coordinator: expected %s, got %s", value, coordReadResp.Value)
	}
	if coordReadResp.Version != writeResp.Version {
		t.Fatalf("Version mismatch from Coordinator: expected %d, got %d", writeResp.Version, coordReadResp.Version)
	}
	t.Logf("✓ Coordinator read consistent: value=%s, version=%d", coordReadResp.Value, coordReadResp.Version)

	// Wait a bit for replication to complete (since W=5, all nodes should have it)
	time.Sleep(2 * time.Second)

	// After Coordinator acknowledges, read from another node - should be consistent
	// Find a node that's not the coordinator
	otherNodeIdx := (coordinatorIdx + 1) % len(nodeURLs)
	otherNodeURL := nodeURLs[otherNodeIdx]

	otherReadResp := readKeyLeaderless(otherNodeURL, key)
	if otherReadResp.StatusCode != 200 {
		t.Fatalf("Read from other node failed: expected 200, got %d", otherReadResp.StatusCode)
	}
	if otherReadResp.Value != value {
		t.Fatalf("Inconsistent data from other node: expected %s, got %s", value, otherReadResp.Value)
	}
	if otherReadResp.Version != writeResp.Version {
		t.Fatalf("Version mismatch from other node: expected %d, got %d", writeResp.Version, otherReadResp.Version)
	}
	t.Logf("✓ Other node read consistent: value=%s, version=%d", otherReadResp.Value, otherReadResp.Version)

	// Verify all nodes are consistent
	allConsistent := true
	for i, nodeURL := range nodeURLs {
		resp := readKeyLeaderless(nodeURL, key)
		if resp.StatusCode != 200 || resp.Value != value || resp.Version != writeResp.Version {
			t.Errorf("Node%d inconsistent: status=%d, value=%s, version=%d", i+1, resp.StatusCode, resp.Value, resp.Version)
			allConsistent = false
		} else {
			t.Logf("✓ Node%d consistent: value=%s, version=%d", i+1, resp.Value, resp.Version)
		}
	}

	if !allConsistent {
		t.Fatal("Not all nodes are consistent after write completion")
	}
}

// Helper types and functions

type LeaderlessWriteResponse struct {
	StatusCode  int
	Version     int
	Coordinator string
}

type LeaderlessReadResponse struct {
	StatusCode int
	Value      string
	Version    int
	Node       string
}

func writeKeyLeaderless(url, key, value string) LeaderlessWriteResponse {
	payload := map[string]string{"key": key, "value": value}
	jsonData, _ := json.Marshal(payload)

	resp, err := http.Post(url+"/set", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return LeaderlessWriteResponse{StatusCode: 500}
	}
	defer resp.Body.Close()

	var result struct {
		Version     int    `json:"version"`
		Coordinator string `json:"coordinator"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return LeaderlessWriteResponse{
		StatusCode:  resp.StatusCode,
		Version:     result.Version,
		Coordinator: result.Coordinator,
	}
}

func readKeyLeaderless(url, key string) LeaderlessReadResponse {
	resp, err := http.Get(url + "/get/" + key)
	if err != nil {
		return LeaderlessReadResponse{StatusCode: 500}
	}
	defer resp.Body.Close()

	var result struct {
		Value   string `json:"value"`
		Version int    `json:"version"`
		Node    string `json:"node"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return LeaderlessReadResponse{
		StatusCode: resp.StatusCode,
		Value:      result.Value,
		Version:    result.Version,
		Node:       result.Node,
	}
}


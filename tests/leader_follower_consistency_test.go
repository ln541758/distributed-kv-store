package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

const (
	leaderURL = "http://localhost:8080"
)

var followerURLs = []string{
	"http://localhost:8081",
	"http://localhost:8082",
	"http://localhost:8083",
	"http://localhost:8084",
}

// TestLeaderConsistencyAfterWrite tests that after Leader acknowledges write,
// reading from Leader returns consistent data
func TestLeaderConsistencyAfterWrite(t *testing.T) {
	key := fmt.Sprintf("test_key_leader_%d", time.Now().UnixNano())
	value := "test_value_leader"

	// Send set to Leader Node
	writeResp := writeKeyLF(leaderURL, key, value)
	if writeResp.StatusCode != 201 {
		t.Fatalf("Write failed: expected 201, got %d", writeResp.StatusCode)
	}
	t.Logf("Write acknowledged by Leader: version %d", writeResp.Version)

	// After Leader acknowledges, read from Leader - should be consistent
	readResp := readKeyLF(leaderURL, key)
	if readResp.StatusCode != 200 {
		t.Fatalf("Read from Leader failed: expected 200, got %d", readResp.StatusCode)
	}
	if readResp.Value != value {
		t.Fatalf("Inconsistent data from Leader: expected %s, got %s", value, readResp.Value)
	}
	if readResp.Version != writeResp.Version {
		t.Fatalf("Version mismatch: expected %d, got %d", writeResp.Version, readResp.Version)
	}
	t.Logf("✓ Leader read consistent: value=%s, version=%d", readResp.Value, readResp.Version)
}

// TestFollowerConsistencyAfterWrite tests that after Leader acknowledges write,
// reading from Followers returns consistent data
func TestFollowerConsistencyAfterWrite(t *testing.T) {
	key := fmt.Sprintf("test_key_follower_%d", time.Now().UnixNano())
	value := "test_value_follower"

	// Send set to Leader Node
	writeResp := writeKeyLF(leaderURL, key, value)
	if writeResp.StatusCode != 201 {
		t.Fatalf("Write failed: expected 201, got %d", writeResp.StatusCode)
	}
	t.Logf("Write acknowledged by Leader: version %d", writeResp.Version)

	// Wait for replication to complete
	time.Sleep(2 * time.Second)

	// After Leader acknowledges, read from Followers - should be consistent
	for i, followerURL := range followerURLs {
		readResp := readKeyLF(followerURL, key)
		if readResp.StatusCode != 200 {
			t.Errorf("Read from Follower%d failed: expected 200, got %d", i+1, readResp.StatusCode)
			continue
		}
		if readResp.Value != value {
			t.Errorf("Inconsistent data from Follower%d: expected %s, got %s", i+1, value, readResp.Value)
			continue
		}
		if readResp.Version != writeResp.Version {
			t.Errorf("Version mismatch from Follower%d: expected %d, got %d", i+1, writeResp.Version, readResp.Version)
			continue
		}
		t.Logf("✓ Follower%d read consistent: value=%s, version=%d", i+1, readResp.Value, readResp.Version)
	}
}

// TestLeaderFollowerInconsistencyWindow tests that during a set operation,
// local_read on Followers might return inconsistent data
func TestLeaderFollowerInconsistencyWindow(t *testing.T) {
	key := fmt.Sprintf("test_key_inconsistency_%d", time.Now().UnixNano())
	value := "test_value_inconsistency"

	type InconsistentRead struct {
		Follower  string
		Status    string
		Value     string
		Version   int
		Timestamp time.Time
	}

	inconsistentReads := []InconsistentRead{}
	var mu sync.Mutex

	// Start write operation in a goroutine
	var writeResp WriteResponseLF
	var writeErr error
	writeDone := make(chan bool)

	go func() {
		writeResp = writeKeyLF(leaderURL, key, value)
		writeErr = nil
		if writeResp.StatusCode != 201 {
			writeErr = fmt.Errorf("write failed with status %d", writeResp.StatusCode)
		}
		writeDone <- true
	}()

	// While write is in progress, repeatedly read from followers using local_read
	readStartTime := time.Now()

	// Start reading immediately (don't wait for write to complete)
	for time.Since(readStartTime) < 3*time.Second {
		for i, followerURL := range followerURLs {
			resp := localReadKeyLF(followerURL, key)

			// Check for inconsistency
			if resp.StatusCode == 404 {
				mu.Lock()
				inconsistentReads = append(inconsistentReads, InconsistentRead{
					Follower:  fmt.Sprintf("Follower%d", i+1),
					Status:    "NOT_FOUND",
					Value:     "",
					Version:   0,
					Timestamp: time.Now(),
				})
				mu.Unlock()
				t.Logf("  [Inconsistency detected] Follower%d: Key not found", i+1)
			} else if resp.StatusCode == 200 && resp.Value != value {
				mu.Lock()
				inconsistentReads = append(inconsistentReads, InconsistentRead{
					Follower:  fmt.Sprintf("Follower%d", i+1),
					Status:    "OLD_VALUE",
					Value:     resp.Value,
					Version:   resp.Version,
					Timestamp: time.Now(),
				})
				mu.Unlock()
				t.Logf("  [Inconsistency detected] Follower%d: Old value '%s' (expected '%s')", i+1, resp.Value, value)
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for write to complete
	<-writeDone

	if writeErr != nil {
		t.Fatalf("Write operation failed: %v", writeErr)
	}

	t.Logf("Write completed: version %d", writeResp.Version)
	t.Logf("Captured %d inconsistent reads during replication window", len(inconsistentReads))

	if len(inconsistentReads) > 0 {
		t.Logf("✓ Successfully detected inconsistency window with %d inconsistent reads", len(inconsistentReads))
		t.Logf("  This demonstrates that during replication, followers may have stale data")
	} else {
		t.Logf("⚠ No inconsistency detected - replication may have been too fast")
		t.Logf("  Try increasing load or network delays to observe inconsistency window")
	}
}

// Helper types and functions

type WriteResponseLF struct {
	StatusCode int
	Version    int
}

type ReadResponseLF struct {
	StatusCode int
	Value      string
	Version    int
}

func writeKeyLF(url, key, value string) WriteResponseLF {
	payload := map[string]string{"key": key, "value": value}
	jsonData, _ := json.Marshal(payload)

	resp, err := http.Post(url+"/set", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return WriteResponseLF{StatusCode: 500}
	}
	defer resp.Body.Close()

	var result struct {
		Version int `json:"version"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return WriteResponseLF{
		StatusCode: resp.StatusCode,
		Version:    result.Version,
	}
}

func readKeyLF(url, key string) ReadResponseLF {
	resp, err := http.Get(url + "/get/" + key)
	if err != nil {
		return ReadResponseLF{StatusCode: 500}
	}
	defer resp.Body.Close()

	var result struct {
		Value   string `json:"value"`
		Version int    `json:"version"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return ReadResponseLF{
		StatusCode: resp.StatusCode,
		Value:      result.Value,
		Version:    result.Version,
	}
}

func localReadKeyLF(url, key string) ReadResponseLF {
	resp, err := http.Get(url + "/local_read/" + key)
	if err != nil {
		return ReadResponseLF{StatusCode: 500}
	}
	defer resp.Body.Close()

	var result struct {
		Value   string `json:"value"`
		Version int    `json:"version"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return ReadResponseLF{
		StatusCode: resp.StatusCode,
		Value:      result.Value,
		Version:    result.Version,
	}
}

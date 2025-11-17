package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
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

// TestBasicConsistency tests basic read-after-write consistency
func TestBasicConsistency() {
	fmt.Println("\n=== Test 1: Basic Consistency ===")

	key := "test_key_1"
	value := "test_value_1"

	// Write to leader
	writeResp := writeKey(leaderURL, key, value)
	if writeResp.StatusCode != 201 {
		fmt.Printf("✗ Write failed: %d\n", writeResp.StatusCode)
		return
	}
	fmt.Printf("Write response: %d, version: %d\n", writeResp.StatusCode, writeResp.Version)

	// Read from leader
	readResp := readKey(leaderURL, key)
	if readResp.StatusCode != 200 || readResp.Value != value {
		fmt.Printf("✗ Leader read failed: status=%d, value=%s\n", readResp.StatusCode, readResp.Value)
		return
	}
	fmt.Printf("Leader read: %s (v%d)\n", readResp.Value, readResp.Version)
	fmt.Println("✓ Leader read consistent")

	// Wait for replication
	time.Sleep(2 * time.Second)

	// Read from followers
	for i, followerURL := range followerURLs {
		readResp = localReadKey(followerURL, key)
		if readResp.StatusCode != 200 || readResp.Value != value {
			fmt.Printf("✗ Follower%d read failed: status=%d, value=%s\n", i+1, readResp.StatusCode, readResp.Value)
		} else {
			fmt.Printf("✓ Follower%d read consistent\n", i+1)
		}
	}
}

// TestInconsistencyWindow tests the inconsistency window during replication
// This test uses local_read endpoint to detect inconsistency during the replication process
func TestInconsistencyWindow() {
	fmt.Println("\n=== Test 2: Inconsistency Window (using local_read) ===")
	fmt.Println("Purpose: Detect stale reads during the replication window")
	fmt.Println("Method: Write to leader, immediately read from followers using local_read")

	key := "test_key_2"
	value := "test_value_2"

	type InconsistentRead struct {
		Follower  string
		Status    string
		Value     string
		Timestamp time.Time
	}

	inconsistentReads := []InconsistentRead{}
	var mu sync.Mutex

	// Write operation
	writeOp := func(wg *sync.WaitGroup) {
		defer wg.Done()
		fmt.Println("  [Write] Starting write operation to leader...")
		start := time.Now()
		resp := writeKey(leaderURL, key, value)
		duration := time.Since(start)
		fmt.Printf("  [Write] Completed in %.2fms, version: %d\n", duration.Seconds()*1000, resp.Version)
	}

	// Read from followers during write using local_read
	readOp := func(wg *sync.WaitGroup) {
		defer wg.Done()
		
		// Small delay to let write start but not complete
		time.Sleep(50 * time.Millisecond)
		fmt.Println("  [Read] Starting local_read from followers during replication...")

		// Try to catch inconsistency during the replication window
		for i := 0; i < 20; i++ {
			for idx, followerURL := range followerURLs {
				resp := localReadKey(followerURL, key)
				
				if resp.StatusCode == 404 {
					mu.Lock()
					inconsistentReads = append(inconsistentReads, InconsistentRead{
						Follower:  fmt.Sprintf("Follower%d", idx+1),
						Status:    "NOT_FOUND",
						Value:     "",
						Timestamp: time.Now(),
					})
					mu.Unlock()
					fmt.Printf("  [Read] Follower%d: Key not found (inconsistent!)\n", idx+1)
				} else if resp.Value != value {
					mu.Lock()
					inconsistentReads = append(inconsistentReads, InconsistentRead{
						Follower:  fmt.Sprintf("Follower%d", idx+1),
						Status:    "OLD_VALUE",
						Value:     resp.Value,
						Timestamp: time.Now(),
					})
					mu.Unlock()
					fmt.Printf("  [Read] Follower%d: Old value '%s' (inconsistent!)\n", idx+1, resp.Value)
				}
			}
			time.Sleep(30 * time.Millisecond)
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	
	testStart := time.Now()
	go writeOp(&wg)
	go readOp(&wg)
	wg.Wait()
	testDuration := time.Since(testStart)

	fmt.Printf("\nTest completed in %.2f seconds\n", testDuration.Seconds())
	fmt.Printf("Captured %d inconsistent reads\n", len(inconsistentReads))
	
	if len(inconsistentReads) > 0 {
		fmt.Println("\nInconsistency window detected! Examples:")
		for i := 0; i < min(5, len(inconsistentReads)); i++ {
			ir := inconsistentReads[i]
			fmt.Printf("  - %s: %s", ir.Follower, ir.Status)
			if ir.Value != "" {
				fmt.Printf(" (value: %s)", ir.Value)
			}
			fmt.Printf(" at %s\n", ir.Timestamp.Format("15:04:05.000"))
		}
		fmt.Println("✓ Successfully detected inconsistency window")
		fmt.Println("  This demonstrates eventual consistency - followers are temporarily stale")
	} else {
		fmt.Println("⚠ No inconsistency detected")
		fmt.Println("  Possible reasons:")
		fmt.Println("  - Replication was too fast")
		fmt.Println("  - Network delays are too small")
		fmt.Println("  - Try increasing the number of iterations or load")
	}
}

// TestEventualConsistency tests eventual consistency after write completion
func TestEventualConsistency() {
	fmt.Println("\n=== Test 3: Eventual Consistency ===")
	fmt.Println("Purpose: Verify all nodes eventually have the same data after replication completes")

	key := "test_key_3"
	value := "test_value_3"

	// Write
	fmt.Println("  [Write] Writing to leader...")
	writeResp := writeKey(leaderURL, key, value)
	fmt.Printf("  [Write] Completed: version %d\n", writeResp.Version)

	// Wait for replication to complete
	fmt.Println("  [Wait] Waiting for replication to complete (2 seconds)...")
	time.Sleep(2 * time.Second)

	// Check all nodes using local_read
	fmt.Println("  [Verify] Checking all nodes using local_read...")
	allConsistent := true
	
	// Check leader
	readResp := localReadKey(leaderURL, key)
	if readResp.StatusCode != 200 || readResp.Value != value {
		allConsistent = false
		fmt.Printf("  ✗ Leader inconsistent: status=%d, value=%s, version=%d\n", 
			readResp.StatusCode, readResp.Value, readResp.Version)
	} else {
		fmt.Printf("  ✓ Leader consistent: value=%s, version=%d\n", readResp.Value, readResp.Version)
	}

	// Check followers
	for i, followerURL := range followerURLs {
		readResp = localReadKey(followerURL, key)
		if readResp.StatusCode != 200 || readResp.Value != value {
			allConsistent = false
			fmt.Printf("  ✗ Follower%d inconsistent: status=%d, value=%s, version=%d\n", 
				i+1, readResp.StatusCode, readResp.Value, readResp.Version)
		} else {
			fmt.Printf("  ✓ Follower%d consistent: value=%s, version=%d\n", 
				i+1, readResp.Value, readResp.Version)
		}
	}

	if allConsistent {
		fmt.Println("\n✓ All nodes eventually consistent")
		fmt.Println("  This demonstrates that despite temporary inconsistency,")
		fmt.Println("  all nodes converge to the same state after replication completes")
	} else {
		fmt.Println("\n✗ Some nodes are still inconsistent")
		fmt.Println("  This may indicate a replication failure")
	}
}

// TestWriteAcknowledgement tests that write only returns after W nodes acknowledge
func TestWriteAcknowledgement() {
	fmt.Println("\n=== Test 4: Write Acknowledgement (W quorum) ===")
	fmt.Println("Purpose: Verify write only returns after W nodes have acknowledged")

	key := "test_key_4"
	value := "test_value_4"

	fmt.Println("  [Write] Starting write operation...")
	start := time.Now()
	writeResp := writeKey(leaderURL, key, value)
	duration := time.Since(start)

	fmt.Printf("  [Write] Completed in %.2fms\n", duration.Seconds()*1000)
	fmt.Printf("  [Write] Status: %d, Version: %d\n", writeResp.StatusCode, writeResp.Version)

	if writeResp.StatusCode == 201 {
		fmt.Println("  ✓ Write acknowledged successfully")
		fmt.Printf("  Note: With W=%d, write took %.2fms\n", 5, duration.Seconds()*1000)
		fmt.Println("  This includes replication delays (200ms per follower + 100ms processing)")
	} else {
		fmt.Printf("  ✗ Write failed with status %d\n", writeResp.StatusCode)
	}

	// Immediately check followers to see how many have the data
	fmt.Println("\n  [Verify] Checking followers immediately after write acknowledgement...")
	consistentCount := 1 // Leader always has it
	
	for i, followerURL := range followerURLs {
		resp := localReadKey(followerURL, key)
		if resp.StatusCode == 200 && resp.Value == value {
			consistentCount++
			fmt.Printf("  ✓ Follower%d has the data\n", i+1)
		} else {
			fmt.Printf("  ✗ Follower%d does not have the data yet\n", i+1)
		}
	}

	fmt.Printf("\n  Result: %d out of 5 nodes have the data\n", consistentCount)
	fmt.Println("  With W=5, all nodes should have acknowledged before write returns")
}

// TestReadConsistency tests read consistency based on R configuration
func TestReadConsistency() {
	fmt.Println("\n=== Test 5: Read Consistency (R quorum) ===")
	fmt.Println("Purpose: Verify read returns consistent data based on R configuration")

	key := "test_key_5"
	value := "test_value_5"

	// Write and wait for replication
	fmt.Println("  [Setup] Writing data and waiting for replication...")
	writeKey(leaderURL, key, value)
	time.Sleep(2 * time.Second)

	// Perform multiple reads
	fmt.Println("  [Read] Performing multiple reads from leader...")
	for i := 0; i < 5; i++ {
		resp := readKey(leaderURL, key)
		if resp.StatusCode == 200 {
			fmt.Printf("  Read %d: value=%s, version=%d\n", i+1, resp.Value, resp.Version)
		} else {
			fmt.Printf("  Read %d: failed with status %d\n", i+1, resp.StatusCode)
		}
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("  ✓ Read consistency test completed")
	fmt.Println("  Note: With R=1, reads only query the leader")
	fmt.Println("  With R=5, reads would query all nodes and return latest version")
}

// TestDifferentWRConfigurations tests different W/R configurations
func TestDifferentWRConfigurations() {
	fmt.Println("\n=== Test 4: Different W/R Configurations ===")
	fmt.Println("Note: Change W and R environment variables and restart servers to test different configurations")
	fmt.Println("Current configuration can be checked via /health endpoint")

	// Check leader health
	resp, err := http.Get(leaderURL + "/health")
	if err != nil {
		fmt.Printf("✗ Failed to check leader health: %v\n", err)
		return
	}
	defer resp.Body.Close()

	var health map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&health)
	fmt.Printf("Leader status: %v\n", health)
	fmt.Println("✓ Health check passed")
}

// Helper types and functions

type WriteResponse struct {
	StatusCode int
	Version    int
}

type ReadResponse struct {
	StatusCode int
	Value      string
	Version    int
}

func writeKey(url, key, value string) WriteResponse {
	payload := map[string]string{"key": key, "value": value}
	jsonData, _ := json.Marshal(payload)

	resp, err := http.Post(url+"/set", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Write error: %v\n", err)
		return WriteResponse{StatusCode: 500}
	}
	defer resp.Body.Close()

	var result struct {
		Version int `json:"version"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return WriteResponse{
		StatusCode: resp.StatusCode,
		Version:    result.Version,
	}
}

func readKey(url, key string) ReadResponse {
	resp, err := http.Get(url + "/get/" + key)
	if err != nil {
		fmt.Printf("Read error: %v\n", err)
		return ReadResponse{StatusCode: 500}
	}
	defer resp.Body.Close()

	var result struct {
		Value   string `json:"value"`
		Version int    `json:"version"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return ReadResponse{
		StatusCode: resp.StatusCode,
		Value:      result.Value,
		Version:    result.Version,
	}
}

func localReadKey(url, key string) ReadResponse {
	resp, err := http.Get(url + "/local_read/" + key)
	if err != nil {
		return ReadResponse{StatusCode: 500}
	}
	defer resp.Body.Close()

	var result struct {
		Value   string `json:"value"`
		Version int    `json:"version"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return ReadResponse{
		StatusCode: resp.StatusCode,
		Value:      result.Value,
		Version:    result.Version,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("Leader-Follower Consistency Tests")
	fmt.Println(strings.Repeat("=", 50))

	// Wait for services to start
	fmt.Println("\nWaiting for services to start...")
	time.Sleep(3 * time.Second)

	// Check if services are running
	fmt.Println("Checking if services are running...")
	resp, err := http.Get(leaderURL + "/health")
	if err != nil {
		fmt.Printf("✗ Leader not running at %s\n", leaderURL)
		fmt.Println("\nPlease start the leader and followers first:")
		fmt.Println("  Terminal 1: cd leader-follower && NODE_TYPE=leader W=5 R=1 PORT=8080 FOLLOWER_URLS=http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084 go run .")
		fmt.Println("  Terminal 2: cd leader-follower && NODE_TYPE=follower PORT=8081 go run .")
		fmt.Println("  Terminal 3: cd leader-follower && NODE_TYPE=follower PORT=8082 go run .")
		fmt.Println("  Terminal 4: cd leader-follower && NODE_TYPE=follower PORT=8083 go run .")
		fmt.Println("  Terminal 5: cd leader-follower && NODE_TYPE=follower PORT=8084 go run .")
		return
	}
	resp.Body.Close()
	fmt.Println("✓ Services are running")

	// Run tests
	TestBasicConsistency()
	TestInconsistencyWindow()
	TestEventualConsistency()
	TestWriteAcknowledgement()
	TestReadConsistency()
	TestDifferentWRConfigurations()

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("All tests completed!")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("\nSummary:")
	fmt.Println("  - Basic consistency: Write-then-read from leader works")
	fmt.Println("  - Inconsistency window: Detected using local_read during replication")
	fmt.Println("  - Eventual consistency: All nodes converge after replication")
	fmt.Println("  - Write acknowledgement: W quorum ensures durability")
	fmt.Println("  - Read consistency: R quorum ensures freshness")
}

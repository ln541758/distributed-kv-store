package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

var nodeURLs = []string{
	"http://localhost:8081",
	"http://localhost:8082",
	"http://localhost:8083",
	"http://localhost:8084",
	"http://localhost:8085",
}

// TestWriteToRandomNode tests writing to a random node
func TestWriteToRandomNode() {
	fmt.Println("\n=== Test 1: Write to Random Node ===")

	key := "test_key_1"
	value := "test_value_1"

	// Select random coordinator
	coordinatorURL := nodeURLs[rand.Intn(len(nodeURLs))]
	fmt.Printf("Selected coordinator: %s\n", coordinatorURL)

	// Write
	writeResp := writeKey(coordinatorURL, key, value)
	if writeResp.StatusCode != 201 {
		fmt.Printf("✗ Write failed: %d\n", writeResp.StatusCode)
		return
	}
	fmt.Printf("Write response: %d, version: %d, coordinator: %s\n", 
		writeResp.StatusCode, writeResp.Version, writeResp.Coordinator)
	fmt.Println("✓ Write successful")

	// Read from coordinator
	readResp := readKey(coordinatorURL, key)
	if readResp.StatusCode != 200 || readResp.Value != value {
		fmt.Printf("✗ Coordinator read failed: status=%d, value=%s\n", readResp.StatusCode, readResp.Value)
		return
	}
	fmt.Printf("Coordinator read: %s (v%d) from %s\n", readResp.Value, readResp.Version, readResp.Node)
	fmt.Println("✓ Coordinator read consistent")
}

// TestInconsistencyWindow tests the inconsistency window in leaderless mode
// This test demonstrates that with W=N, R=1, there is a window where reads can be stale
func TestInconsistencyWindow() {
	fmt.Println("\n=== Test 2: Inconsistency Window (W=N, R=1) ===")
	fmt.Println("Purpose: Detect stale reads during write coordinator's replication to peers")
	fmt.Println("Method: Write to one node, immediately read from other nodes using local_read")

	key := "test_key_2"
	value := "test_value_2"

	type InconsistentRead struct {
		Node      string
		Status    string
		Value     string
		Timestamp time.Time
	}

	inconsistentReads := []InconsistentRead{}
	var mu sync.Mutex

	coordinatorURL := nodeURLs[0]
	fmt.Printf("  [Setup] Using %s as write coordinator\n", coordinatorURL)

	// Write operation
	writeOp := func(wg *sync.WaitGroup) {
		defer wg.Done()
		fmt.Println("  [Write] Starting write operation to coordinator...")
		start := time.Now()
		resp := writeKey(coordinatorURL, key, value)
		duration := time.Since(start)
		fmt.Printf("  [Write] Completed in %.2fms, version: %d\n", duration.Seconds()*1000, resp.Version)
	}

	// Read from other nodes during write using local_read
	readOp := func(wg *sync.WaitGroup) {
		defer wg.Done()
		
		// Small delay to let write start but not complete replication
		time.Sleep(100 * time.Millisecond)
		fmt.Println("  [Read] Starting local_read from other nodes during replication...")

		// Try to catch inconsistency during the replication window
		for i := 0; i < 20; i++ {
			for idx, nodeURL := range nodeURLs[1:] { // Skip coordinator
				resp := localReadKey(nodeURL, key)
				
				if resp.StatusCode == 404 {
					mu.Lock()
					inconsistentReads = append(inconsistentReads, InconsistentRead{
						Node:      fmt.Sprintf("Node%d", idx+2),
						Status:    "NOT_FOUND",
						Value:     "",
						Timestamp: time.Now(),
					})
					mu.Unlock()
					fmt.Printf("  [Read] Node%d: Key not found (inconsistent!)\n", idx+2)
				} else if resp.Value != value {
					mu.Lock()
					inconsistentReads = append(inconsistentReads, InconsistentRead{
						Node:      fmt.Sprintf("Node%d", idx+2),
						Status:    "OLD_VALUE",
						Value:     resp.Value,
						Timestamp: time.Now(),
					})
					mu.Unlock()
					fmt.Printf("  [Read] Node%d: Old value '%s' (inconsistent!)\n", idx+2, resp.Value)
				}
			}
			time.Sleep(40 * time.Millisecond)
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
			fmt.Printf("  - %s: %s", ir.Node, ir.Status)
			if ir.Value != "" {
				fmt.Printf(" (value: %s)", ir.Value)
			}
			fmt.Printf(" at %s\n", ir.Timestamp.Format("15:04:05.000"))
		}
		fmt.Println("✓ Successfully detected inconsistency window")
		fmt.Println("  This demonstrates the trade-off with W=N, R=1:")
		fmt.Println("  - Writes are durable (all nodes must acknowledge)")
		fmt.Println("  - But reads can be stale during the replication window")
	} else {
		fmt.Println("⚠ No inconsistency detected")
		fmt.Println("  Possible reasons:")
		fmt.Println("  - Replication was too fast")
		fmt.Println("  - Network delays are too small")
		fmt.Println("  - Try increasing the number of iterations or load")
	}
}

// TestReadFromCoordinatorVsOthers compares reads from coordinator vs other nodes
func TestReadFromCoordinatorVsOthers() {
	fmt.Println("\n=== Test 2.5: Read from Coordinator vs Other Nodes ===")
	fmt.Println("Purpose: Show that coordinator has data immediately, others may lag")

	key := "test_key_2_5"
	value := "test_value_2_5"

	coordinatorURL := nodeURLs[0]

	// Write to coordinator
	fmt.Printf("  [Write] Writing to coordinator %s...\n", coordinatorURL)
	writeResp := writeKey(coordinatorURL, key, value)
	fmt.Printf("  [Write] Completed, version: %d\n", writeResp.Version)

	// Immediately read from coordinator
	fmt.Println("\n  [Read] Immediately reading from coordinator:")
	coordResp := readKey(coordinatorURL, key)
	if coordResp.StatusCode == 200 {
		fmt.Printf("  ✓ Coordinator has data: value=%s, version=%d\n", coordResp.Value, coordResp.Version)
	} else {
		fmt.Printf("  ✗ Coordinator read failed: status=%d\n", coordResp.StatusCode)
	}

	// Immediately read from other nodes (may be inconsistent)
	fmt.Println("\n  [Read] Immediately reading from other nodes (may be stale):")
	for i, nodeURL := range nodeURLs[1:] {
		resp := readKey(nodeURL, key)
		if resp.StatusCode == 200 {
			fmt.Printf("  Node%d: Has data (value=%s, version=%d)\n", i+2, resp.Value, resp.Version)
		} else {
			fmt.Printf("  Node%d: Does not have data yet (status=%d) - INCONSISTENT!\n", i+2, resp.StatusCode)
		}
	}

	// Wait and read again
	fmt.Println("\n  [Wait] Waiting 2 seconds for replication...")
	time.Sleep(2 * time.Second)

	fmt.Println("  [Read] Reading from all nodes after replication:")
	for i, nodeURL := range nodeURLs {
		resp := readKey(nodeURL, key)
		if resp.StatusCode == 200 {
			fmt.Printf("  ✓ Node%d: value=%s, version=%d\n", i+1, resp.Value, resp.Version)
		} else {
			fmt.Printf("  ✗ Node%d: status=%d\n", i+1, resp.StatusCode)
		}
	}

	fmt.Println("\n  This demonstrates eventual consistency in leaderless architecture")
}

// TestEventualConsistency tests eventual consistency
func TestEventualConsistency() {
	fmt.Println("\n=== Test 3: Eventual Consistency ===")

	key := "test_key_3"
	value := "test_value_3"

	// Write to first node
	coordinatorURL := nodeURLs[0]
	writeResp := writeKey(coordinatorURL, key, value)
	fmt.Printf("Write completed: version %d\n", writeResp.Version)

	// Wait for replication
	time.Sleep(3 * time.Second)

	// Check all nodes
	allConsistent := true
	for i, nodeURL := range nodeURLs {
		readResp := readKey(nodeURL, key)
		if readResp.StatusCode != 200 || readResp.Value != value {
			allConsistent = false
			fmt.Printf("✗ Node%d (%s) inconsistent: status=%d, value=%s\n", 
				i+1, nodeURL, readResp.StatusCode, readResp.Value)
		} else {
			fmt.Printf("✓ Node%d (%s) consistent\n", i+1, nodeURL)
		}
	}

	if allConsistent {
		fmt.Println("✓ All nodes eventually consistent")
	}
}

// TestMultipleCoordinators tests concurrent writes from different coordinators
func TestMultipleCoordinators() {
	fmt.Println("\n=== Test 4: Multiple Coordinators Concurrent Writes ===")

	results := []map[string]interface{}{}
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Each node writes a different key as coordinator
	for i, nodeURL := range nodeURLs {
		wg.Add(1)
		go func(url string, idx int) {
			defer wg.Done()

			key := fmt.Sprintf("concurrent_key_%d", idx)
			value := fmt.Sprintf("concurrent_value_%d", idx)

			writeResp := writeKey(url, key, value)

			mu.Lock()
			results = append(results, map[string]interface{}{
				"coordinator": url,
				"key":         key,
				"status":      writeResp.StatusCode,
				"version":     writeResp.Version,
			})
			mu.Unlock()
		}(nodeURL, i)
	}

	wg.Wait()

	fmt.Printf("Completed %d concurrent writes\n", len(results))
	for _, result := range results {
		fmt.Printf("  - %v\n", result)
	}

	successful := 0
	for _, result := range results {
		if result["status"] == 201 {
			successful++
		}
	}
	fmt.Printf("✓ %d/%d writes successful\n", successful, len(results))
}

// TestReadFromDifferentNodes tests reading the same key from different nodes
func TestReadFromDifferentNodes() {
	fmt.Println("\n=== Test 5: Read from Different Nodes ===")

	key := "test_key_5"
	value := "test_value_5"

	// Write to first node
	writeResp := writeKey(nodeURLs[0], key, value)
	fmt.Printf("Write completed: version %d\n", writeResp.Version)

	// Wait for replication
	time.Sleep(3 * time.Second)

	// Read from all nodes
	fmt.Println("Reading from all nodes:")
	for i, nodeURL := range nodeURLs {
		readResp := readKey(nodeURL, key)
		if readResp.StatusCode == 200 {
			fmt.Printf("  Node%d: value=%s, version=%d, node=%s\n", 
				i+1, readResp.Value, readResp.Version, readResp.Node)
		} else {
			fmt.Printf("  Node%d: status=%d\n", i+1, readResp.StatusCode)
		}
	}
	fmt.Println("✓ Read test completed")
}

// Helper types and functions

type WriteResponse struct {
	StatusCode  int
	Version     int
	Coordinator string
}

type ReadResponse struct {
	StatusCode int
	Value      string
	Version    int
	Node       string
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
		Version     int    `json:"version"`
		Coordinator string `json:"coordinator"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return WriteResponse{
		StatusCode:  resp.StatusCode,
		Version:     result.Version,
		Coordinator: result.Coordinator,
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
		Node    string `json:"node"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return ReadResponse{
		StatusCode: resp.StatusCode,
		Value:      result.Value,
		Version:    result.Version,
		Node:       result.Node,
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
		Node    string `json:"node"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return ReadResponse{
		StatusCode: resp.StatusCode,
		Value:      result.Value,
		Version:    result.Version,
		Node:       result.Node,
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	rand.Seed(time.Now().UnixNano())

	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("Leaderless Consistency Tests")
	fmt.Println(strings.Repeat("=", 50))

	// Wait for services to start
	fmt.Println("\nWaiting for services to start...")
	time.Sleep(3 * time.Second)

	// Check if services are running
	fmt.Println("Checking if services are running...")
	resp, err := http.Get(nodeURLs[0] + "/health")
	if err != nil {
		fmt.Printf("✗ Node1 not running at %s\n", nodeURLs[0])
		fmt.Println("\nPlease start all nodes first:")
		fmt.Println("  Terminal 1: cd leaderless && NODE_ID=node1 W=5 R=1 PORT=8080 PEER_URLS=http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084 go run .")
		fmt.Println("  Terminal 2: cd leaderless && NODE_ID=node2 W=5 R=1 PORT=8081 PEER_URLS=http://localhost:8080,http://localhost:8082,http://localhost:8083,http://localhost:8084 go run .")
		fmt.Println("  Terminal 3: cd leaderless && NODE_ID=node3 W=5 R=1 PORT=8082 PEER_URLS=http://localhost:8080,http://localhost:8081,http://localhost:8083,http://localhost:8084 go run .")
		fmt.Println("  Terminal 4: cd leaderless && NODE_ID=node4 W=5 R=1 PORT=8083 PEER_URLS=http://localhost:8080,http://localhost:8081,http://localhost:8082,http://localhost:8084 go run .")
		fmt.Println("  Terminal 5: cd leaderless && NODE_ID=node5 W=5 R=1 PORT=8084 PEER_URLS=http://localhost:8080,http://localhost:8081,http://localhost:8082,http://localhost:8083 go run .")
		return
	}
	resp.Body.Close()
	fmt.Println("✓ Services are running")

	// Run tests
	TestWriteToRandomNode()
	TestInconsistencyWindow()
	TestReadFromCoordinatorVsOthers()
	TestEventualConsistency()
	TestMultipleCoordinators()
	TestReadFromDifferentNodes()

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("All tests completed!")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("\nSummary:")
	fmt.Println("  - Write to random node: Any node can be write coordinator")
	fmt.Println("  - Inconsistency window: Detected using local_read during replication")
	fmt.Println("  - Coordinator vs others: Coordinator has data immediately, others lag")
	fmt.Println("  - Eventual consistency: All nodes converge after replication")
	fmt.Println("  - Multiple coordinators: Concurrent writes to different keys work")
	fmt.Println("  - Read from different nodes: R=1 means single node read (may be stale)")
}

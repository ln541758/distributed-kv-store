package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// LoadTester manages load testing operations
type LoadTester struct {
	mode               string
	urls               []string
	numKeys            int
	writeLatencies     []float64
	readLatencies      []float64
	staleReads         []StaleRead
	readWriteIntervals []float64
	versions           map[string]VersionInfo
	keyAccessTimes     map[string][]AccessInfo
	mu                 sync.Mutex
}

// VersionInfo tracks version and timestamp for a key
type VersionInfo struct {
	Version   int
	Timestamp time.Time
}

// AccessInfo tracks access time and type
type AccessInfo struct {
	Timestamp time.Time
	OpType    string // "read" or "write"
}

// StaleRead represents a stale read event
type StaleRead struct {
	Key             string  `json:"key"`
	ExpectedVersion int     `json:"expected_version"`
	ActualVersion   int     `json:"actual_version"`
	TimeSinceWrite  float64 `json:"time_since_write"`
}

// NewLoadTester creates a new load tester
func NewLoadTester(mode string, urls []string, numKeys int) *LoadTester {
	return &LoadTester{
		mode:           mode,
		urls:           urls,
		numKeys:        numKeys,
		versions:       make(map[string]VersionInfo),
		keyAccessTimes: make(map[string][]AccessInfo),
	}
}

// WriteOperation performs a write operation
func (lt *LoadTester) WriteOperation(key, value string) (float64, bool) {
	url := lt.urls[0]
	if lt.mode == "leaderless" {
		url = lt.urls[rand.Intn(len(lt.urls))]
	}

	payload := map[string]string{
		"key":   key,
		"value": value,
	}
	jsonData, _ := json.Marshal(payload)

	start := time.Now()
	resp, err := http.Post(url+"/set", "application/json", bytes.NewBuffer(jsonData))
	latency := time.Since(start).Seconds()

	if err != nil {
		return latency, false
	}
	defer resp.Body.Close()

	if resp.StatusCode == 201 {
		var result struct {
			Version int `json:"version"`
		}
		json.NewDecoder(resp.Body).Decode(&result)

		lt.mu.Lock()
		lt.versions[key] = VersionInfo{
			Version:   result.Version,
			Timestamp: time.Now(),
		}
		lt.keyAccessTimes[key] = append(lt.keyAccessTimes[key], AccessInfo{
			Timestamp: time.Now(),
			OpType:    "write",
		})
		lt.mu.Unlock()

		return latency, true
	}

	return latency, false
}

// ReadOperation performs a read operation
func (lt *LoadTester) ReadOperation(key string) (float64, bool) {
	// Leader-Follower: ALL reads go to leader (who coordinates with R nodes)
	// Leaderless: Reads go to any random node (local read only)
	url := lt.urls[0]
	if lt.mode == "leaderless" {
		url = lt.urls[rand.Intn(len(lt.urls))]
	}

	start := time.Now()
	resp, err := http.Get(url + "/get/" + key)
	latency := time.Since(start).Seconds()

	if err != nil {
		return latency, false
	}
	defer resp.Body.Close()

	// Check for staleness before processing response
	isStale := false
	lt.mu.Lock()
	vInfo, keyExists := lt.versions[key]
	lt.mu.Unlock()

	if resp.StatusCode == 200 {
		var result struct {
			Value   string `json:"value"`
			Version int    `json:"version"`
		}
		json.NewDecoder(resp.Body).Decode(&result)

		// Check if version is stale
		lt.mu.Lock()
		if keyExists {
			if result.Version < vInfo.Version {
				isStale = true
				lt.staleReads = append(lt.staleReads, StaleRead{
					Key:             key,
					ExpectedVersion: vInfo.Version,
					ActualVersion:   result.Version,
					TimeSinceWrite:  time.Since(vInfo.Timestamp).Seconds(),
				})
			}
		}
		lt.keyAccessTimes[key] = append(lt.keyAccessTimes[key], AccessInfo{
			Timestamp: time.Now(),
			OpType:    "read",
		})
		lt.mu.Unlock()

		return latency, isStale
	}

	// If we get 404 but we know the key should exist (we just wrote it),
	// this is a stale read - the node hasn't received replication yet
	if resp.StatusCode == 404 && keyExists {
		isStale = true
		lt.mu.Lock()
		lt.staleReads = append(lt.staleReads, StaleRead{
			Key:             key,
			ExpectedVersion: vInfo.Version,
			ActualVersion:   0, // Key doesn't exist on this node yet
			TimeSinceWrite:  time.Since(vInfo.Timestamp).Seconds(),
		})
		lt.keyAccessTimes[key] = append(lt.keyAccessTimes[key], AccessInfo{
			Timestamp: time.Now(),
			OpType:    "read",
		})
		lt.mu.Unlock()
	}

	return latency, isStale
}

// GenerateWorkload generates the test workload
func (lt *LoadTester) GenerateWorkload(duration int, writeRatio float64, opsPerSecond int) {
	fmt.Printf("\nStarting load test:\n")
	fmt.Printf("  Mode: %s\n", lt.mode)
	fmt.Printf("  Duration: %d seconds\n", duration)
	fmt.Printf("  Write ratio: %.0f%%\n", writeRatio*100)
	fmt.Printf("  Read ratio: %.0f%%\n", (1-writeRatio)*100)
	fmt.Printf("  Target QPS: %d\n", opsPerSecond)

	startTime := time.Now()
	operationCount := 0
	var wg sync.WaitGroup

	// Key pool for generating locality
	keyPool := make([]string, lt.numKeys)
	for i := 0; i < lt.numKeys; i++ {
		keyPool[i] = fmt.Sprintf("key_%d", i)
	}

	ticker := time.NewTicker(time.Second / time.Duration(opsPerSecond))
	defer ticker.Stop()

	endTime := startTime.Add(time.Duration(duration) * time.Second)

	for time.Now().Before(endTime) {
		<-ticker.C

		// Decide operation type
		isWrite := rand.Float64() < writeRatio

		// Select key
		key := keyPool[rand.Intn(len(keyPool))]

		operationCount++
		wg.Add(1)

		// Fire operation asynchronously to allow concurrency
		go func(k string, write bool) {
			defer wg.Done()

			if write {
				value := fmt.Sprintf("value_%d", time.Now().UnixNano())
				latency, _ := lt.WriteOperation(k, value)
				lt.mu.Lock()
				lt.writeLatencies = append(lt.writeLatencies, latency)
				lt.mu.Unlock()
			} else {
				latency, _ := lt.ReadOperation(k)
				lt.mu.Lock()
				lt.readLatencies = append(lt.readLatencies, latency)
				lt.mu.Unlock()
			}
		}(key, isWrite)
	}

	fmt.Printf("Fired %d operations, waiting for completion...\n", operationCount)
	wg.Wait()
	fmt.Printf("Completed %d operations\n", operationCount)
}

// CalculateIntervals calculates read-write intervals for the same key
func (lt *LoadTester) CalculateIntervals() {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	for _, accesses := range lt.keyAccessTimes {
		if len(accesses) < 2 {
			continue
		}

		// Sort by timestamp
		sort.Slice(accesses, func(i, j int) bool {
			return accesses[i].Timestamp.Before(accesses[j].Timestamp)
		})

		// Calculate intervals between consecutive operations
		for i := 0; i < len(accesses)-1; i++ {
			if accesses[i].OpType != accesses[i+1].OpType {
				interval := accesses[i+1].Timestamp.Sub(accesses[i].Timestamp).Seconds()
				lt.readWriteIntervals = append(lt.readWriteIntervals, interval)
			}
		}
	}
}

// PrintStatistics prints test statistics
func (lt *LoadTester) PrintStatistics() {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Load Test Results")
	fmt.Println(strings.Repeat("=", 60))

	// Write latency statistics
	if len(lt.writeLatencies) > 0 {
		fmt.Printf("\nWrite Operations (%d total):\n", len(lt.writeLatencies))
		fmt.Printf("  Average latency: %.2fms\n", mean(lt.writeLatencies)*1000)
		fmt.Printf("  Median latency: %.2fms\n", median(lt.writeLatencies)*1000)
		fmt.Printf("  P95 latency: %.2fms\n", percentile(lt.writeLatencies, 95)*1000)
		fmt.Printf("  P99 latency: %.2fms\n", percentile(lt.writeLatencies, 99)*1000)
		fmt.Printf("  Max latency: %.2fms\n", max(lt.writeLatencies)*1000)
	}

	// Read latency statistics
	if len(lt.readLatencies) > 0 {
		fmt.Printf("\nRead Operations (%d total):\n", len(lt.readLatencies))
		fmt.Printf("  Average latency: %.2fms\n", mean(lt.readLatencies)*1000)
		fmt.Printf("  Median latency: %.2fms\n", median(lt.readLatencies)*1000)
		fmt.Printf("  P95 latency: %.2fms\n", percentile(lt.readLatencies, 95)*1000)
		fmt.Printf("  P99 latency: %.2fms\n", percentile(lt.readLatencies, 99)*1000)
		fmt.Printf("  Max latency: %.2fms\n", max(lt.readLatencies)*1000)
	}

	// Stale reads
	fmt.Printf("\nStale Reads:\n")
	fmt.Printf("  Total: %d\n", len(lt.staleReads))
	if len(lt.readLatencies) > 0 {
		staleRate := float64(len(lt.staleReads)) / float64(len(lt.readLatencies)) * 100
		fmt.Printf("  Rate: %.2f%%\n", staleRate)
	}

	if len(lt.staleReads) > 0 {
		fmt.Println("  Examples:")
		for i := 0; i < minInt(3, len(lt.staleReads)); i++ {
			sr := lt.staleReads[i]
			fmt.Printf("    - Key: %s, Expected: v%d, Actual: v%d, Time since write: %.2fms\n",
				sr.Key, sr.ExpectedVersion, sr.ActualVersion, sr.TimeSinceWrite*1000)
		}
	}

	// Read-write intervals
	lt.CalculateIntervals()
	if len(lt.readWriteIntervals) > 0 {
		fmt.Printf("\nRead-Write Intervals (%d total):\n", len(lt.readWriteIntervals))
		fmt.Printf("  Average interval: %.2fms\n", mean(lt.readWriteIntervals)*1000)
		fmt.Printf("  Median interval: %.2fms\n", median(lt.readWriteIntervals)*1000)
		fmt.Printf("  Min interval: %.2fms\n", minFloat(lt.readWriteIntervals)*1000)
		fmt.Printf("  Max interval: %.2fms\n", max(lt.readWriteIntervals)*1000)
	}
}

// SaveResults saves results to JSON file
func (lt *LoadTester) SaveResults(filename string) error {
	results := map[string]interface{}{
		"mode":                 lt.mode,
		"write_latencies":      lt.writeLatencies,
		"read_latencies":       lt.readLatencies,
		"stale_reads":          lt.staleReads,
		"read_write_intervals": lt.readWriteIntervals,
		"statistics": map[string]interface{}{
			"total_writes":      len(lt.writeLatencies),
			"total_reads":       len(lt.readLatencies),
			"total_stale_reads": len(lt.staleReads),
			"write_avg_latency": mean(lt.writeLatencies),
			"read_avg_latency":  mean(lt.readLatencies),
		},
	}

	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(results); err != nil {
		return err
	}

	fmt.Printf("\nResults saved to: %s\n", filename)
	return nil
}

// Helper functions
func mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

func median(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)
	return sorted[len(sorted)/2]
}

func percentile(data []float64, p float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)
	index := int(float64(len(sorted)) * p / 100.0)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func max(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	maxVal := data[0]
	for _, v := range data {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

func minFloat(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	minVal := data[0]
	for _, v := range data {
		if v < minVal {
			minVal = v
		}
	}
	return minVal
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func min(nums ...interface{}) interface{} {
	switch v := nums[0].(type) {
	case int:
		minVal := v
		for _, num := range nums[1:] {
			if n, ok := num.(int); ok && n < minVal {
				minVal = n
			}
		}
		return minVal
	case []float64:
		if len(v) == 0 {
			return 0.0
		}
		minVal := v[0]
		for _, num := range v {
			if num < minVal {
				minVal = num
			}
		}
		return minVal
>>>>>>> 4177218 (Update leaderless)
	}
	minVal := data[0]
	for _, v := range data {
		if v < minVal {
			minVal = v
		}
	}
	return minVal
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	mode := flag.String("mode", "leader", "Database mode: leader or leaderless")
	writeRatio := flag.Float64("write-ratio", 0.5, "Write ratio (0.0-1.0)")
	duration := flag.Int("duration", 60, "Test duration in seconds")
	qps := flag.Int("qps", 10, "Operations per second")
	numKeys := flag.Int("num-keys", 50, "Number of keys (smaller = more conflicts)")
	output := flag.String("output", "results.json", "Output filename")

	flag.Parse()

	// Configure URLs
	var urls []string
	if *mode == "leader" {
		// For leader-follower, include leader + all followers
		// Writes go to leader, reads distributed across all nodes
		urls = []string{
			"http://localhost:8080", // Leader
			"http://localhost:8081", // Follower 1
			"http://localhost:8082", // Follower 2
			"http://localhost:8083", // Follower 3
			"http://localhost:8084", // Follower 4
		}
	} else {
		urls = []string{
			"http://localhost:8081",
			"http://localhost:8082",
			"http://localhost:8083",
			"http://localhost:8084",
			"http://localhost:8085",
		}
	}

	// Create tester
	tester := NewLoadTester(*mode, urls, *numKeys)

	// Run test
	tester.GenerateWorkload(*duration, *writeRatio, *qps)

	// Print and save results
	tester.PrintStatistics()
	if err := tester.SaveResults(*output); err != nil {
		log.Fatal(err)
	}
}

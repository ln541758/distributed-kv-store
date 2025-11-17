# Load Testing Guide for Distributed KV Store

## Overview

This guide explains how to perform comprehensive load testing on your distributed key-value store implementations (Leader-Follower and Leaderless) to analyze read-write performance, latency distributions, and stale read detection.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Load Testing Architecture](#load-testing-architecture)
3. [How to Run Tests](#how-to-run-tests)
4. [Understanding the Results](#understanding-the-results)
5. [Visualization](#visualization)
6. [Load Test Generator Design](#load-test-generator-design)

---

## Quick Start

### Prerequisites

```bash
# Install Python dependencies for visualization
pip3 install -r requirements.txt

# Ensure Go is installed
go version  # Should be 1.21+
```

### Run All Tests

```bash
# Leader-Follower (interactive menu)
./scripts/run_leader_follower.sh

# Leaderless
./scripts/run_leaderless.sh

# Generate visualizations
./scripts/visualize_results.py
```

---

## Load Testing Architecture

### Load Tester Features

The load tester (`load-tester/main.go`) implements:

1. **Version Tracking**: Tracks the latest version of each key written
2. **Stale Read Detection**: Compares read versions against expected versions
3. **Latency Measurement**: Records latency for every read and write operation
4. **Temporal Locality**: Uses a small key pool to ensure reads/writes cluster on the same keys
5. **Read-Write Interval Tracking**: Measures time between consecutive operations on the same key

### Key Parameters

```bash
go run main.go [flags]

Flags:
  -mode string          # "leader" or "leaderless" (default: "leader")
  -write-ratio float    # Write percentage 0.0-1.0 (default: 0.5)
  -duration int         # Test duration in seconds (default: 60)
  -qps int             # Operations per second (default: 10)
  -num-keys int        # Size of key pool (default: 50)
  -output string       # Output JSON filename (default: "results.json")
```

### Test Configurations

The scripts test **4 read-write ratios**:

| Configuration | Writes | Reads | Use Case |
|--------------|--------|-------|----------|
| Read-Heavy   | 1%     | 99%   | Caching, static content |
| Read-Dominant| 10%    | 90%   | Social media feeds |
| Balanced     | 50%    | 50%   | General purpose |
| Write-Heavy  | 90%    | 10%   | Logging, metrics collection |

### Leader-Follower Configurations

Tests **3 W/R configurations** for each read-write ratio:

| Config | W | R | Description | Trade-offs |
|--------|---|---|-------------|-----------|
| W=5, R=1 | 5 | 1 | Strong write durability | Slower writes, fast reads (may be stale) |
| W=1, R=5 | 1 | 5 | Fast writes, consistent reads | Faster writes, slower reads |
| W=3, R=3 | 3 | 3 | Quorum consistency | Balanced performance |

### Leaderless Configuration

Tests **W=5, R=1** (write to all nodes, read from any one) for each read-write ratio.

---

## How to Run Tests

### Option 1: Automated Scripts

#### Leader-Follower

```bash
cd /path/to/distributed-kv-store
./scripts/run_leader_follower.sh
```

Interactive menu will appear:
```
Select configuration to test:
1) W=5, R=1 (Strong write consistency)
2) W=1, R=5 (Fast writes, consistent reads)
3) W=3, R=3 (Quorum)
4) Run all configurations
5) Exit
```

#### Leaderless

```bash
./scripts/run_leaderless.sh
```

Automatically runs all 4 read-write ratios.

### Option 2: Manual Testing

#### Step 1: Start Servers

**Leader-Follower:**
```bash
# Terminal 1 - Leader
cd leader-follower
NODE_TYPE=leader W=5 R=1 PORT=8080 \
  FOLLOWER_URLS=http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084 \
  go run .

# Terminals 2-5 - Followers
NODE_TYPE=follower PORT=8081 go run .
NODE_TYPE=follower PORT=8082 go run .
NODE_TYPE=follower PORT=8083 go run .
NODE_TYPE=follower PORT=8084 go run .
```

**Leaderless:**
```bash
# Terminal 1-5 - All nodes
cd leaderless
NODE_ID=node1 W=5 R=1 PORT=8081 PEER_URLS=http://localhost:8082,http://localhost:8083,http://localhost:8084,http://localhost:8085 go run .
# ... (repeat for nodes 2-5 with different ports)
```

#### Step 2: Run Load Tests

```bash
cd load-tester

# Example: 10% writes, 90% reads, 60 seconds
go run main.go \
  -mode leader \
  -write-ratio 0.10 \
  -duration 60 \
  -qps 20 \
  -num-keys 50 \
  -output results_test.json
```

---

## Understanding the Results

### Result Files

Tests generate JSON files with naming pattern:
```
results_<mode>_<config>_<write>w<read>r.json
```

Examples:
- `results_leader_w5r1_10w90r.json` - Leader-Follower, W=5/R=1, 10% writes
- `results_leaderless_50w50r.json` - Leaderless, 50/50 split

### JSON Structure

```json
{
  "mode": "leader",
  "write_latencies": [0.0023, 0.0025, ...],  // seconds
  "read_latencies": [0.0012, 0.0015, ...],   // seconds
  "stale_reads": [
    {
      "key": "key_5",
      "expected_version": 10,
      "actual_version": 8,
      "time_since_write": 0.0045  // seconds
    }
  ],
  "read_write_intervals": [0.0032, 0.0041, ...],  // seconds
  "statistics": {
    "total_writes": 120,
    "total_reads": 1080,
    "total_stale_reads": 23,
    "write_avg_latency": 0.0024,
    "read_avg_latency": 0.0013
  }
}
```

### Console Output

During test execution, you'll see:
```
Starting load test:
  Mode: leader
  Duration: 30 seconds
  Write ratio: 10%
  Read ratio: 90%
  Target QPS: 20

Completed 600 operations

============================================================
Load Test Results
============================================================

Write Operations (60 total):
  Average latency: 2.34ms
  Median latency: 2.12ms
  P95 latency: 4.56ms
  P99 latency: 7.89ms
  Max latency: 12.34ms

Read Operations (540 total):
  Average latency: 1.23ms
  Median latency: 1.01ms
  P95 latency: 2.34ms
  P99 latency: 3.45ms
  Max latency: 5.67ms

Stale Reads:
  Total: 15
  Rate: 2.78%

Read-Write Intervals (245 total):
  Average interval: 3.45ms
  Median interval: 2.89ms
  Min interval: 0.12ms
  Max interval: 15.67ms

Results saved to: results_leader_w5r1_10w90r.json
```

---

## Visualization

### Generate Graphs

```bash
# Generate all visualizations from all result files
./scripts/visualize_results.py

# Or specify specific files
./scripts/visualize_results.py results_leader_*.json
```

### Generated Outputs

All files saved to `visualizations/` directory:

#### 1. Latency Distribution Graphs

For each test, generates:
- `<config>_read_latency.png` - Read latency histogram + CDF
- `<config>_write_latency.png` - Write latency histogram + CDF

**What to look for:**
- **Long tail**: CDF plot shows if latencies have a long tail (steep at the end)
- **Percentiles**: P50, P95, P99 marked clearly
- **Outliers**: Max latency shown in stats box

#### 2. Read-Write Interval Distribution

- `<config>_intervals.png` - Time between read/write on same key

**What to look for:**
- **Temporal locality**: Should show short intervals (clustering)
- **Distribution shape**: Should be concentrated at low values

#### 3. Comparison Graphs

- `comparison_p99_latency.png` - P99 latencies across all configurations
- `comparison_stale_reads.png` - Stale read rates across configurations

#### 4. Summary Report

- `summary_report.txt` - Text file with all statistics

---

## Load Test Generator Design

### How It Works

#### 1. **Temporal Locality via Small Key Pool**

```go
numKeys := 50  // Small pool ensures frequent collisions
keyPool := make([]string, numKeys)
for i := 0; i < numKeys; i++ {
    keyPool[i] = fmt.Sprintf("key_%d", i)
}
```

**Why this works:**
- With 50 keys and 600 operations (30s @ 20 QPS), average 12 operations per key
- Random selection ensures some keys accessed much more frequently
- Creates natural clustering of reads and writes on same keys

#### 2. **Version Tracking**

```go
type VersionInfo struct {
    Version   int
    Timestamp time.Time
}

versions map[string]VersionInfo  // Tracks latest known version per key
```

**On Write:**
```go
// Server returns new version
resp.Version = 5

// Client updates tracking
versions[key] = VersionInfo{
    Version:   5,
    Timestamp: time.Now()
}
```

**On Read:**
```go
// Server returns data with version
resp.Version = 4
resp.Value = "data"

// Client checks for staleness
if resp.Version < versions[key].Version {
    // STALE READ DETECTED!
    staleReads = append(staleReads, StaleRead{
        Key:             key,
        ExpectedVersion: versions[key].Version,
        ActualVersion:   resp.Version,
        TimeSinceWrite:  time.Since(versions[key].Timestamp)
    })
}
```

#### 3. **Interval Tracking**

```go
type AccessInfo struct {
    Timestamp time.Time
    OpType    string  // "read" or "write"
}

keyAccessTimes map[string][]AccessInfo  // All accesses per key
```

After test:
```go
for key, accesses := range keyAccessTimes {
    sort.By(accesses, Timestamp)
    
    // Calculate intervals between different operation types
    for i := 0; i < len(accesses)-1; i++ {
        if accesses[i].OpType != accesses[i+1].OpType {
            interval := accesses[i+1].Timestamp - accesses[i].Timestamp
            readWriteIntervals = append(readWriteIntervals, interval)
        }
    }
}
```

### Guaranteeing Read-Write Clustering

The load test guarantees temporal locality through:

1. **Small Key Space**: 50 keys vs potentially 600 operations = high collision rate
2. **Random Selection**: Uniform random ensures some keys hit multiple times quickly
3. **Statistical Properties**: 
   - Expected operations per key: 12 (600 / 50)
   - Some keys will see 20+ operations due to random variance
   - Time between operations on popular keys: milliseconds

**Example Timeline for key_5:**
```
0.00s: Write key_5 = "val1" (v1)
0.15s: Write key_5 = "val2" (v2)
0.23s: Read key_5 → might get v1 (stale!) or v2
0.45s: Write key_5 = "val3" (v3)
0.47s: Read key_5 → might get v1, v2, or v3
```

**Intervals tracked:** 0.15s, 0.08s, 0.22s, 0.02s

### Alternative Approach: Larger Key Space with Zipfian Distribution

For more realistic workloads:

```go
// Zipfian distribution: some keys very popular, most rarely accessed
func ZipfianKey(n int, s float64) string {
    // s = 1.0 typical for real workloads
    // Most accesses to small subset of keys
}
```

Could be implemented but adds complexity. Current approach is simpler and still effective.

---

## Interpreting Results

### Expected Patterns

#### Leader-Follower W=5, R=1
- **Write latency**: Higher (must wait for 5 nodes)
- **Read latency**: Lower (only 1 node)
- **Stale reads**: Higher rate (reading from 1 node may be behind)

#### Leader-Follower W=1, R=5
- **Write latency**: Lower (only 1 node acknowledgment)
- **Read latency**: Higher (query 5 nodes, pick latest)
- **Stale reads**: Lower rate (quorum of 5 likely has latest)

#### Leader-Follower W=3, R=3
- **Write/Read latency**: Balanced
- **Stale reads**: Medium rate

#### Leaderless W=5, R=1
- **Write latency**: Highest (all nodes must acknowledge)
- **Read latency**: Lowest (any single node)
- **Stale reads**: Potentially highest (replication lag)

### Red Flags

- **P99 >> P95**: Indicates significant outliers, investigate
- **Stale reads > 10%**: Replication might be too slow
- **Write latency > 100ms**: Network or database bottleneck
- **Bimodal distribution**: System switching between two states

### Success Criteria

- **P99 latency < 50ms**: Good performance
- **Stale read rate < 5%**: Acceptable for read-heavy workloads
- **Short intervals (< 10ms mean)**: Good temporal locality

---

## Troubleshooting

### "Connection refused" errors
```bash
# Check if servers are running
lsof -i :8080-8085

# Increase sleep time in scripts
sleep 10  # instead of sleep 5
```

### High stale read rates
- Increase W value
- Increase R value
- Add delay between write and read
- Check replication lag in server logs

### No stale reads detected
- Decrease num-keys (more collisions)
- Increase QPS (more load)
- Verify version tracking is working in servers

### Memory issues
- Reduce duration or QPS
- Clear old result files
- Increase available memory

---

## Advanced Usage

### Custom Test Scenarios

```bash
# High load, short keys
go run main.go -qps 100 -num-keys 10 -duration 120

# Low collision rate
go run main.go -num-keys 1000 -qps 10

# Write-only test
go run main.go -write-ratio 1.0

# Read-only test (after pre-populating)
go run main.go -write-ratio 0.0
```

### Analyzing Specific Patterns

```python
import json
import matplotlib.pyplot as plt

with open('results.json') as f:
    data = json.load(f)

# Plot write latency over time (order matters)
plt.plot(data['write_latencies'])
plt.xlabel('Operation Number')
plt.ylabel('Latency (s)')
plt.title('Latency Over Time')
plt.show()
```

---

## Summary

Your load testing infrastructure is **complete and production-ready**:

✅ Load tester with version tracking and stale read detection  
✅ Automated test scripts for all configurations  
✅ Temporal locality through small key pool  
✅ Comprehensive visualization scripts  
✅ Detailed result reporting  

**Next Steps:**
1. Run tests: `./scripts/run_leader_follower.sh`
2. Generate graphs: `./scripts/visualize_results.py`
3. Analyze results in `visualizations/` directory
4. Write your discussion comparing configurations

Good luck with your load testing! 🚀


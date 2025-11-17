# Distributed Key-Value Database

A distributed key-value database implementation in Go with Leader-Follower and Leaderless architectures, featuring comprehensive load testing and performance analysis tools.

## 🚀 Quick Start

**Run all load tests and generate visualizations:**
```bash
./scripts/run_all_tests.sh
```

See **[QUICK_START.md](QUICK_START.md)** for more options.

## 📁 Project Structure

```
.
├── leader-follower/          # Leader-Follower implementation
│   ├── main.go              # Entry point
│   ├── kv_store.go          # Core KV store logic with versioning
│   ├── server.go            # HTTP server
│   └── go.mod
├── leaderless/              # Leaderless (Dynamo-style) implementation
│   ├── main.go              # Entry point
│   ├── kv_store.go          # Core KV store with vector clocks
│   ├── server.go            # HTTP server
│   └── go.mod
├── load-tester/             # Load testing tool
│   ├── main.go              # Load tester with stale read detection
│   └── go.mod
├── tests/                   # Unit and integration tests
│   ├── leader_follower_test.go
│   └── leaderless_test.go
├── scripts/                 # Automated test scripts
│   ├── run_all_tests.sh            # Master script (all configs)
│   ├── run_leader_follower.sh      # Leader-Follower tests
│   ├── run_leaderless.sh           # Leaderless tests
│   ├── visualize_results.py        # Graph generator
│   ├── check_setup.sh              # Setup verification
│   ├── deploy.sh
│   └── cleanup.sh
├── results/                 # JSON test results (generated)
├── visualizations/          # Graphs and reports (generated)
├── QUICK_START.md           # Quick reference guide
├── LOAD_TESTING_GUIDE.md    # Comprehensive testing documentation
├── LOCAL_TESTING.md         # Manual testing instructions
├── IMPLEMENTATION_SUMMARY.md # What was implemented
├── requirements.txt         # Python dependencies for visualization
└── README.md
```

## 📋 Prerequisites

- **Go 1.21+** - For running the KV store servers
- **Python 3.7+** - For visualization (optional)
- **matplotlib & numpy** - For graphs (install via `pip3 install -r requirements.txt`)

## 🎯 Usage Options

### Option 1: Run All Tests (Recommended)

```bash
# Runs all configurations and generates graphs (~45 minutes)
./scripts/run_all_tests.sh
```

This executes:
- 3 Leader-Follower configs (W=5/R=1, W=1/R=5, W=3/R=3)
- 1 Leaderless config (W=5/R=1)
- 4 read-write ratios each (1/99, 10/90, 50/50, 90/10)
- Automatic visualization generation

### Option 2: Individual Tests

**Leader-Follower (Interactive Menu):**
```bash
./scripts/run_leader_follower.sh
# Select: W=5/R=1, W=1/R=5, W=3/R=3, or all
```

**Leaderless:**
```bash
./scripts/run_leaderless.sh
```

### Option 3: Manual Testing

See **[LOCAL_TESTING.md](LOCAL_TESTING.md)** for manual startup instructions.

## 📊 Load Testing

### Test Configurations

The load tester evaluates performance across multiple dimensions:

| Aspect | Configurations |
|--------|----------------|
| **Read-Write Ratios** | 1/99, 10/90, 50/50, 90/10 |
| **Leader-Follower** | W=5/R=1, W=1/R=5, W=3/R=3 |
| **Leaderless** | W=5/R=1 |

### Metrics Collected

✅ **Latency** - Mean, median, P95, P99, max for reads and writes  
✅ **Stale Reads** - Detects and tracks version mismatches  
✅ **Read-Write Intervals** - Time between operations on same key  
✅ **Long Tail Analysis** - CDF plots show latency distributions  

### Temporal Locality

The load tester ensures reads and writes cluster on the same keys through:
- **Small key pool** (default: 50 keys)
- **Random selection** creates natural hotspots
- **High collision rate** guarantees temporal clustering

### Example Output

```
Write Operations (60 total):
  Average latency: 2.34ms
  P95 latency: 4.56ms
  P99 latency: 7.89ms

Read Operations (540 total):
  Average latency: 1.23ms
  P99 latency: 3.45ms

Stale Reads: 15 (2.78%)
```

## 📈 Visualization

### Generate Graphs

```bash
# Install dependencies first
pip3 install -r requirements.txt

# Generate all visualizations
./scripts/visualize_results.py
```

### Generated Outputs

All saved to `visualizations/` directory:

1. **Latency Distributions** - Histogram + CDF for reads and writes
   - Shows long tail clearly
   - Marks P50, P95, P99 percentiles

2. **Read-Write Intervals** - Distribution of time between operations on same key

3. **Comparison Graphs** - P99 latencies and stale read rates across configs

4. **Summary Report** - `summary_report.txt` with complete statistics

## 🏗️ Architecture

### Leader-Follower

- **Leader**: Receives writes, replicates to W followers
- **Followers**: Serve reads, R followers queried (return most recent)
- **Versioning**: Monotonically increasing version numbers
- **Consistency**: Tunable via W and R parameters

### Leaderless

- **All nodes equal**: Any node can coordinate reads/writes
- **Replication**: Write coordinator sends to W nodes
- **Read repair**: Query R nodes, return most recent version
- **Versioning**: Vector clocks for conflict detection
- **Consistency**: Tunable via W and R parameters

## 📚 Documentation

- **[QUICK_START.md](QUICK_START.md)** - Quick reference and common commands
- **[LOAD_TESTING_GUIDE.md](LOAD_TESTING_GUIDE.md)** - Complete testing documentation
- **[LOCAL_TESTING.md](LOCAL_TESTING.md)** - Manual testing and development

## 🔬 API Reference

### Write Operation

```bash
curl -X POST http://localhost:8080/set \
  -H 'Content-Type: application/json' \
  -d '{"key":"mykey","value":"myvalue"}'

# Response
{
  "message": "Key set successfully",
  "version": 5
}
```

### Read Operation

```bash
curl http://localhost:8080/get/mykey

# Response
{
  "key": "mykey",
  "value": "myvalue",
  "version": 5
}
```

### Local Read (Testing Only)

```bash
curl http://localhost:8081/local_read/mykey

# Response (from single node, may be stale)
{
  "key": "mykey",
  "value": "myvalue",
  "version": 4
}
```

## 🧪 Running Tests

```bash
cd tests

# Leader-Follower tests
go run leader_follower_test.go

# Leaderless tests
go run leaderless_test.go
```

## 🎛️ Configuration

### Environment Variables

**Leader-Follower:**
- `NODE_TYPE` - "leader" or "follower"
- `PORT` - Server port (default: 8080)
- `W` - Write quorum size (leader only)
- `R` - Read quorum size (leader only)
- `FOLLOWER_URLS` - Comma-separated follower URLs (leader only)

**Leaderless:**
- `NODE_ID` - Unique node identifier
- `PORT` - Server port
- `W` - Write quorum size
- `R` - Read quorum size
- `PEER_URLS` - Comma-separated peer URLs

### Load Tester Flags

```bash
go run main.go [flags]

  -mode string          "leader" or "leaderless"
  -write-ratio float    0.0 to 1.0 (e.g., 0.10 = 10% writes)
  -duration int         Test duration in seconds
  -qps int             Operations per second
  -num-keys int        Key pool size (smaller = more conflicts)
  -output string       Output JSON filename
```

## 🔍 Troubleshooting

### Ports in Use

```bash
# Check what's using the ports
lsof -i :8080-8085

# Kill stuck processes
lsof -ti:8080,8081,8082,8083,8084,8085 | xargs kill -9
```

### Python Visualization Errors

```bash
# Install or upgrade dependencies
pip3 install --upgrade matplotlib numpy
```

### No Stale Reads Detected

- Reduce key pool size: `-num-keys 20`
- Increase load: `-qps 50`
- Verify version tracking in server logs

## 🎓 Understanding Results

### Expected Patterns

| Configuration | Write Latency | Read Latency | Stale Reads |
|--------------|--------------|--------------|-------------|
| W=5, R=1 | High | Low | High (2-10%) |
| W=1, R=5 | Low | High | Low (<1%) |
| W=3, R=3 | Medium | Medium | Medium (1-5%) |
| Leaderless W=5, R=1 | Highest | Lowest | Varies |

### Long Tail

- P99 typically 2-5x median (normal)
- Max can be 10x+ (network delays, GC pauses)
- CDF plots show distribution shape

### Trade-offs

**W=5, R=1**: Strong durability, fast reads, may be stale  
**W=1, R=5**: Fast writes, consistent reads, slower reads  
**W=3, R=3**: Quorum consistency, balanced performance  

## 📝 License

This project is for educational purposes.

## 🙏 Acknowledgments

Implements concepts from:
- Dynamo: Amazon's Highly Available Key-value Store
- Chain Replication for Supporting High Throughput and Availability
- Consistency trade-offs in modern distributed database system design
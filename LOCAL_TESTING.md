# Local Testing Guide

This guide explains how to test the distributed KV store locally without Docker or Terraform.

## Prerequisites

- Go 1.21 or higher
- Multiple terminal windows

## Leader-Follower Testing

### Step 1: Start the Nodes

Open 5 terminal windows and run:

**Terminal 1 - Leader (W=5, R=1):**
```bash
cd leader-follower
NODE_TYPE=leader W=5 R=1 PORT=8080 \
  FOLLOWER_URLS=http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084 \
  go run .
```

**Terminal 2 - Follower 1:**
```bash
cd leader-follower
NODE_TYPE=follower PORT=8081 go run .
```

**Terminal 3 - Follower 2:**
```bash
cd leader-follower
NODE_TYPE=follower PORT=8082 go run .
```

**Terminal 4 - Follower 3:**
```bash
cd leader-follower
NODE_TYPE=follower PORT=8083 go run .
```

**Terminal 5 - Follower 4:**
```bash
cd leader-follower
NODE_TYPE=follower PORT=8084 go run .
```

### Step 2: Run Tests

In a 6th terminal:
```bash
cd tests
go run leader_follower_test.go
```

### Step 3: Manual Testing

```bash
# Write a key
curl -X POST http://localhost:8080/set \
  -H 'Content-Type: application/json' \
  -d '{"key":"mykey","value":"myvalue"}'

# Read from leader
curl http://localhost:8080/get/mykey

# Local read from follower (for testing inconsistency)
curl http://localhost:8081/local_read/mykey
```

### Different W/R Configurations

**W=1, R=5 (Fast writes, consistent reads):**
```bash
NODE_TYPE=leader W=1 R=5 PORT=8080 \
  FOLLOWER_URLS=http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084 \
  go run .
```

**W=3, R=3 (Quorum):**
```bash
NODE_TYPE=leader W=3 R=3 PORT=8080 \
  FOLLOWER_URLS=http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084 \
  go run .
```

## Leaderless Testing

### Step 1: Start the Nodes

Open 5 terminal windows and run:

**Terminal 1 - Node 1:**
```bash
cd leaderless
NODE_ID=node1 W=5 R=1 PORT=8080 \
  PEER_URLS=http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084 \
  go run .
```

**Terminal 2 - Node 2:**
```bash
cd leaderless
NODE_ID=node2 W=5 R=1 PORT=8081 \
  PEER_URLS=http://localhost:8080,http://localhost:8082,http://localhost:8083,http://localhost:8084 \
  go run .
```

**Terminal 3 - Node 3:**
```bash
cd leaderless
NODE_ID=node3 W=5 R=1 PORT=8082 \
  PEER_URLS=http://localhost:8080,http://localhost:8081,http://localhost:8083,http://localhost:8084 \
  go run .
```

**Terminal 4 - Node 4:**
```bash
cd leaderless
NODE_ID=node4 W=5 R=1 PORT=8083 \
  PEER_URLS=http://localhost:8080,http://localhost:8081,http://localhost:8082,http://localhost:8084 \
  go run .
```

**Terminal 5 - Node 5:**
```bash
cd leaderless
NODE_ID=node5 W=5 R=1 PORT=8084 \
  PEER_URLS=http://localhost:8080,http://localhost:8081,http://localhost:8082,http://localhost:8083 \
  go run .
```

### Step 2: Run Tests

In a 6th terminal:
```bash
cd tests
go run leaderless_test.go
```

### Step 3: Manual Testing

```bash
# Write to any node (it becomes the coordinator)
curl -X POST http://localhost:8080/set \
  -H 'Content-Type: application/json' \
  -d '{"key":"mykey","value":"myvalue"}'

# Read from any node
curl http://localhost:8081/get/mykey

# Local read from another node (for testing inconsistency)
curl http://localhost:8082/local_read/mykey
```

## Load Testing

After starting the nodes, run load tests:

```bash
cd load-tester

# Leader-Follower mode
go run main.go -mode leader -write-ratio 0.5 -duration 60 -qps 20 -output results.json

# Leaderless mode  
go run main.go -mode leaderless -write-ratio 0.5 -duration 60 -qps 20 -output results.json
```

### Load Test Parameters

- `-mode`: "leader" or "leaderless"
- `-write-ratio`: Percentage of writes (0.01 = 1%, 0.5 = 50%, 0.9 = 90%)
- `-duration`: Test duration in seconds
- `-qps`: Operations per second
- `-num-keys`: Number of keys (smaller = more conflicts, default 50)
- `-output`: Output JSON file

### Example Load Tests

```bash
# 1% write, 99% read
go run main.go -mode leader -write-ratio 0.01 -duration 30 -qps 20

# 10% write, 90% read
go run main.go -mode leader -write-ratio 0.10 -duration 30 -qps 20

# 50% write, 50% read
go run main.go -mode leader -write-ratio 0.50 -duration 30 -qps 20

# 90% write, 10% read
go run main.go -mode leader -write-ratio 0.90 -duration 30 -qps 20
```

## Understanding the Tests

### Test 1: Basic Consistency
- Writes to leader/coordinator
- Reads back immediately
- Verifies data is consistent

### Test 2: Inconsistency Window
- **Key test for the assignment**
- Uses `local_read` endpoint to detect stale data
- Writes to one node while simultaneously reading from others
- Demonstrates the inconsistency window during replication
- Shows eventual consistency in action

### Test 3: Eventual Consistency
- Writes data and waits for replication
- Verifies all nodes eventually have the same data
- Uses `local_read` to check each node individually

### Test 4: Write Acknowledgement (Leader-Follower)
- Verifies write only returns after W nodes acknowledge
- Measures write latency based on W configuration

### Test 5: Read Consistency (Leader-Follower)
- Tests read behavior based on R configuration
- Shows difference between R=1 and R=5

## Key Endpoints for Testing

### `/set` - Write Operation
```bash
POST /set
Content-Type: application/json
{"key": "mykey", "value": "myvalue"}

Response: {"version": 1}
```

### `/get/{key}` - Read Operation
```bash
GET /get/mykey

Response: {"value": "myvalue", "version": 1}
```

### `/local_read/{key}` - Local Read (Testing Only)
**This is the key endpoint for testing inconsistency!**

```bash
GET /local_read/mykey

Response: {"value": "myvalue", "version": 1}
```

This endpoint:
- Reads only from the local node's storage
- Does NOT query other nodes
- Used to detect inconsistency during replication
- Shows the actual state of each individual node

### `/health` - Health Check
```bash
GET /health

Response: {"status": "healthy", "node_type": "leader"}
```

## Observing Inconsistency

To manually observe the inconsistency window:

1. Start all nodes
2. In one terminal, write a key:
   ```bash
   curl -X POST http://localhost:8080/set -H 'Content-Type: application/json' -d '{"key":"test","value":"v1"}'
   ```

3. **Immediately** (within 1 second) in another terminal, read from followers:
   ```bash
   # May return 404 or old value during replication
   curl http://localhost:8081/local_read/test
   curl http://localhost:8082/local_read/test
   curl http://localhost:8083/local_read/test
   curl http://localhost:8084/local_read/test
   ```

4. Wait 2 seconds and read again - should be consistent:
   ```bash
   curl http://localhost:8081/local_read/test
   ```

## Simulated Delays

The code includes simulated delays to make inconsistency observable:

- **Leader → Follower replication**: 200ms per message
- **Follower write processing**: 100ms
- **Follower read processing**: 50ms

This means with W=5, a write takes approximately:
- Leader local write: ~0ms
- 4 followers × (200ms network + 100ms processing) = ~1200ms
- **Total: ~1.2 seconds**

During this window, `local_read` from followers will show inconsistency!

## Troubleshooting

### Port Already in Use
```bash
# Find process using port
lsof -i :8080

# Kill process
kill -9 <PID>
```

### Connection Refused
- Make sure all nodes are started
- Check that ports match in FOLLOWER_URLS/PEER_URLS
- Wait a few seconds after starting nodes

### No Inconsistency Detected
- Try running the test multiple times
- Increase the number of iterations in the test
- The timing is sensitive - inconsistency window is brief

## Tips for Assignment

1. **Use `local_read` extensively** - This is how you prove inconsistency exists
2. **Run tests multiple times** - Timing issues may cause tests to miss the window
3. **Take screenshots** - Capture the inconsistency window output
4. **Vary W and R** - Show how different configurations affect consistency
5. **Document observations** - Explain why inconsistency occurs and when it resolves

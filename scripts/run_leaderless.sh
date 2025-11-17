#!/bin/bash

# Script to run Leaderless tests locally

set -e

echo "=========================================="
echo "Leaderless Distributed KV Store"
echo "=========================================="

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Create results directory
RESULTS_DIR="results"
mkdir -p "$RESULTS_DIR"

# Store PIDs for cleanup
declare -a PIDS=()

# Cleanup function
cleanup() {
    echo -e "\n${BLUE}Cleaning up...${NC}"
    for pid in "${PIDS[@]}"; do
        if ps -p $pid > /dev/null 2>&1; then
            kill $pid 2>/dev/null || true
        fi
    done
    wait 2>/dev/null || true
    echo -e "${GREEN}Cleanup complete${NC}"
}

# Set trap for cleanup on exit
trap cleanup EXIT INT TERM

echo -e "\n${BLUE}=== Testing Leaderless (W=5, R=1) ===${NC}"

# Start servers
echo "Starting Leaderless nodes..."
cd leaderless

# Start all 5 nodes
NODE_ID=node1 W=5 R=1 PORT=8081 PEER_URLS=http://localhost:8082,http://localhost:8083,http://localhost:8084,http://localhost:8085 go run . > /tmp/leaderless1.log 2>&1 &
PIDS+=($!)

NODE_ID=node2 W=5 R=1 PORT=8082 PEER_URLS=http://localhost:8081,http://localhost:8083,http://localhost:8084,http://localhost:8085 go run . > /tmp/leaderless2.log 2>&1 &
PIDS+=($!)

NODE_ID=node3 W=5 R=1 PORT=8083 PEER_URLS=http://localhost:8081,http://localhost:8082,http://localhost:8084,http://localhost:8085 go run . > /tmp/leaderless3.log 2>&1 &
PIDS+=($!)

NODE_ID=node4 W=5 R=1 PORT=8084 PEER_URLS=http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8085 go run . > /tmp/leaderless4.log 2>&1 &
PIDS+=($!)

NODE_ID=node5 W=5 R=1 PORT=8085 PEER_URLS=http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084 go run . > /tmp/leaderless5.log 2>&1 &
PIDS+=($!)

cd ..

# Wait for services to start
echo "Waiting for services to start..."
sleep 5

# Run load tests
echo -e "\n${GREEN}Running load tests...${NC}"
cd load-tester

# 1% write / 99% read
echo "Testing 1% write / 99% read..."
go run main.go -mode leaderless -write-ratio 0.01 -duration 30 -qps 20 -num-keys 50 -output "../${RESULTS_DIR}/leaderless_1w99r.json"

# 10% write / 90% read
echo "Testing 10% write / 90% read..."
go run main.go -mode leaderless -write-ratio 0.10 -duration 30 -qps 20 -num-keys 50 -output "../${RESULTS_DIR}/leaderless_10w90r.json"

# 50% write / 50% read
echo "Testing 50% write / 50% read..."
go run main.go -mode leaderless -write-ratio 0.50 -duration 30 -qps 20 -num-keys 50 -output "../${RESULTS_DIR}/leaderless_50w50r.json"

# 90% write / 10% read
echo "Testing 90% write / 10% read..."
go run main.go -mode leaderless -write-ratio 0.90 -duration 30 -qps 20 -num-keys 50 -output "../${RESULTS_DIR}/leaderless_90w10r.json"

cd ..

echo -e "\n${GREEN}All tests completed!${NC}"
echo "Results saved in current directory"

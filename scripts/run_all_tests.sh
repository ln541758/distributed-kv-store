#!/bin/bash

# Master script to run all load tests and generate visualizations
# This runs all Leader-Follower configurations and Leaderless tests

set -e

echo "========================================================================"
echo "DISTRIBUTED KV STORE - COMPLETE LOAD TEST SUITE"
echo "========================================================================"
echo ""
echo "This will run:"
echo "  - 3 Leader-Follower configurations (W=5/R=1, W=1/R=5, W=3/R=3)"
echo "  - 1 Leaderless configuration (W=5/R=1)"
echo "  - 4 read-write ratios each (1/99, 10/90, 50/50, 90/10)"
echo "  - Total: 16 test runs"
echo ""
echo "Estimated time: ~45 minutes (30s per test + setup/teardown)"
echo ""
read -p "Continue? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 0
fi

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

START_TIME=$(date +%s)

# Store PIDs for cleanup
declare -a PIDS=()

# Cleanup function
cleanup() {
    echo -e "\n${BLUE}Cleaning up background processes...${NC}"
    for pid in "${PIDS[@]}"; do
        if ps -p $pid > /dev/null 2>&1; then
            kill $pid 2>/dev/null || true
        fi
    done
    # Also kill any remaining Go processes on our ports
    lsof -ti:8080,8081,8082,8083,8084,8085 | xargs kill -9 2>/dev/null || true
    wait 2>/dev/null || true
}

# Set trap for cleanup on exit
trap cleanup EXIT INT TERM

# Function to run Leader-Follower tests
run_leader_follower() {
    local w=$1
    local r=$2
    local config_name="w${w}r${r}"
    
    echo -e "\n${BLUE}╔═══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║  Leader-Follower: W=${w}, R=${r}${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════════╝${NC}"
    
    # Start servers
    echo "Starting Leader and Follower nodes..."
    cd leader-follower
    
    NODE_TYPE=leader W=$w R=$r PORT=8080 \
        FOLLOWER_URLS=http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084 \
        go run . > /tmp/leader_${config_name}.log 2>&1 &
    PIDS+=($!)
    
    for port in 8081 8082 8083 8084; do
        NODE_TYPE=follower PORT=$port go run . > /tmp/follower_${config_name}_${port}.log 2>&1 &
        PIDS+=($!)
    done
    
    cd ..
    sleep 5
    
    # Run tests
    cd load-tester
    
    for ratio in "0.01:1w99r" "0.10:10w90r" "0.50:50w50r" "0.90:90w10r"; do
        IFS=':' read -r write_ratio name <<< "$ratio"
        echo -e "${YELLOW}→ Testing ${name}...${NC}"
        go run main.go -mode leader -write-ratio $write_ratio -duration 30 -qps 20 -num-keys 50 \
            -output "../${RESULTS_DIR}/leader_${config_name}_${name}.json" 2>/dev/null
        echo -e "${GREEN}  ✓ Completed${NC}"
    done
    
    cd ..
    
    # Cleanup
    cleanup
    PIDS=()
    sleep 2
}

# Function to run Leaderless tests
run_leaderless() {
    echo -e "\n${BLUE}╔═══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║  Leaderless: W=5, R=1${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════════╝${NC}"
    
    # Start servers
    echo "Starting Leaderless nodes..."
    cd leaderless
    
    for i in {1..5}; do
        port=$((8080 + i))
        peers=""
        for j in {1..5}; do
            if [ $j -ne $i ]; then
                peerport=$((8080 + j))
                peers="${peers}http://localhost:${peerport},"
            fi
        done
        peers=${peers%,}  # Remove trailing comma
        
        NODE_ID=node$i W=5 R=1 PORT=$port PEER_URLS=$peers \
            go run . > /tmp/leaderless_node${i}.log 2>&1 &
        PIDS+=($!)
    done
    
    cd ..
    sleep 5
    
    # Run tests
    cd load-tester
    
    for ratio in "0.01:1w99r" "0.10:10w90r" "0.50:50w50r" "0.90:90w10r"; do
        IFS=':' read -r write_ratio name <<< "$ratio"
        echo -e "${YELLOW}→ Testing ${name}...${NC}"
        go run main.go -mode leaderless -write-ratio $write_ratio -duration 30 -qps 20 -num-keys 50 \
            -output "../${RESULTS_DIR}/leaderless_${name}.json" 2>/dev/null
        echo -e "${GREEN}  ✓ Completed${NC}"
    done
    
    cd ..
    
    # Cleanup
    cleanup
    PIDS=()
    sleep 2
}

# Create results directory
RESULTS_DIR="results"
mkdir -p "$RESULTS_DIR"
echo "Results will be saved to: $RESULTS_DIR/"
echo ""

# Main execution
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}Phase 1: Leader-Follower Tests${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"

run_leader_follower 5 1
run_leader_follower 1 5
run_leader_follower 3 3

echo -e "\n${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}Phase 2: Leaderless Tests${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"

run_leaderless

# Generate visualizations
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}Phase 3: Generating Visualizations${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}\n"

if command -v python3 &> /dev/null; then
    # Check if required packages are installed
    if python3 -c "import matplotlib, numpy" 2>/dev/null; then
        ./scripts/visualize_results.py ${RESULTS_DIR}/*.json
    else
        echo -e "${YELLOW}⚠ Python packages not installed. Run:${NC}"
        echo -e "  pip3 install -r requirements.txt"
        echo -e "${YELLOW}Then run:${NC}"
        echo -e "  ./scripts/visualize_results.py ${RESULTS_DIR}/*.json"
    fi
else
    echo -e "${YELLOW}⚠ Python3 not found. Install Python to generate graphs.${NC}"
fi

# Calculate duration
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))
MINUTES=$((DURATION / 60))
SECONDS=$((DURATION % 60))

# Summary
echo -e "\n${GREEN}╔═══════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  ALL TESTS COMPLETED SUCCESSFULLY! ✓                      ║${NC}"
echo -e "${GREEN}╚═══════════════════════════════════════════════════════════╝${NC}\n"

echo "Time taken: ${MINUTES}m ${SECONDS}s"
echo ""
echo "Results:"
echo "  - Test data: ${RESULTS_DIR}/ (16 JSON files)"
echo "  - Visualizations: visualizations/ directory"
echo "  - Summary report: visualizations/summary_report.txt"
echo ""
echo "Next steps:"
echo "  1. Review graphs in visualizations/ directory"
echo "  2. Read summary_report.txt for detailed statistics"
echo "  3. Review raw data in ${RESULTS_DIR}/ directory"
echo "  4. Compare configurations to analyze trade-offs"
echo ""


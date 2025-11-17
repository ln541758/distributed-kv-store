#!/bin/bash

# Script to run Leader-Follower tests with different W/R configurations

set -e

echo "=========================================="
echo "Leader-Follower Distributed KV Store"
echo "=========================================="

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to run tests for a specific W/R configuration
run_test() {
    local w=$1
    local r=$2
    local config_name="w${w}r${r}"
    
    echo -e "\n${BLUE}=== Testing Leader-Follower (W=${w}, R=${r}) ===${NC}"
    
    # Initialize and apply Terraform
    cd terraform
    terraform init -upgrade
    
    # Modify the leader container environment variables
    echo "Deploying infrastructure..."
    terraform apply -auto-approve
    
    cd ..
    
    # Wait for services to start
    echo "Waiting for services to start..."
    sleep 10
    
    # Run unit tests
    echo -e "\n${GREEN}Running unit tests...${NC}"
    cd tests
    go run leader_follower_test.go
    cd ..
    
    # Run load tests with different read/write ratios
    echo -e "\n${GREEN}Running load tests...${NC}"
    cd load-tester
    
    # 1% write / 99% read
    echo "Testing 1% write / 99% read..."
    go run main.go -mode leader -write-ratio 0.01 -duration 30 -qps 20 -output "../results_leader_${config_name}_1w99r.json"
    
    # 10% write / 90% read
    echo "Testing 10% write / 90% read..."
    go run main.go -mode leader -write-ratio 0.10 -duration 30 -qps 20 -output "../results_leader_${config_name}_10w90r.json"
    
    # 50% write / 50% read
    echo "Testing 50% write / 50% read..."
    go run main.go -mode leader -write-ratio 0.50 -duration 30 -qps 20 -output "../results_leader_${config_name}_50w50r.json"
    
    # 90% write / 10% read
    echo "Testing 90% write / 10% read..."
    go run main.go -mode leader -write-ratio 0.90 -duration 30 -qps 20 -output "../results_leader_${config_name}_90w10r.json"
    
    cd ..
    
    # Cleanup
    echo "Cleaning up..."
    cd terraform
    terraform destroy -auto-approve
    cd ..
    
    echo -e "${GREEN}✓ Tests completed for W=${w}, R=${r}${NC}"
}

# Main menu
echo ""
echo "Select configuration to test:"
echo "1) W=5, R=1 (Strong write consistency)"
echo "2) W=1, R=5 (Fast writes, consistent reads)"
echo "3) W=3, R=3 (Quorum)"
echo "4) Run all configurations"
echo "5) Exit"
echo ""
read -p "Enter choice (1-5): " choice

case $choice in
    1)
        run_test 5 1
        ;;
    2)
        run_test 1 5
        ;;
    3)
        run_test 3 3
        ;;
    4)
        run_test 5 1
        run_test 1 5
        run_test 3 3
        ;;
    5)
        echo "Exiting"
        exit 0
        ;;
    *)
        echo -e "${RED}Invalid choice${NC}"
        exit 1
        ;;
esac

echo -e "\n${GREEN}All tests completed!${NC}"
echo "Results saved in current directory"

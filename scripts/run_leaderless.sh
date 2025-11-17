#!/bin/bash

# Script to run Leaderless tests

set -e

echo "=========================================="
echo "Leaderless Distributed KV Store"
echo "=========================================="

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "\n${BLUE}=== Testing Leaderless (W=N, R=1) ===${NC}"

# Deploy infrastructure
cd terraform
echo "Initializing Terraform..."
terraform init -upgrade

echo "Deploying infrastructure..."
terraform apply -auto-approve

cd ..

# Wait for services to start
echo "Waiting for services to start..."
sleep 10

# Run unit tests
echo -e "\n${GREEN}Running unit tests...${NC}"
cd tests
go run leaderless_test.go
cd ..

# Run load tests
echo -e "\n${GREEN}Running load tests...${NC}"
cd load-tester

# 1% write / 99% read
echo "Testing 1% write / 99% read..."
go run main.go -mode leaderless -write-ratio 0.01 -duration 30 -qps 20 -output "../results_leaderless_1w99r.json"

# 10% write / 90% read
echo "Testing 10% write / 90% read..."
go run main.go -mode leaderless -write-ratio 0.10 -duration 30 -qps 20 -output "../results_leaderless_10w90r.json"

# 50% write / 50% read
echo "Testing 50% write / 50% read..."
go run main.go -mode leaderless -write-ratio 0.50 -duration 30 -qps 20 -output "../results_leaderless_50w50r.json"

# 90% write / 10% read
echo "Testing 90% write / 10% read..."
go run main.go -mode leaderless -write-ratio 0.90 -duration 30 -qps 20 -output "../results_leaderless_90w10r.json"

cd ..

# Cleanup
echo "Cleaning up..."
cd terraform
terraform destroy -auto-approve
cd ..

echo -e "\n${GREEN}All tests completed!${NC}"
echo "Results saved in current directory"

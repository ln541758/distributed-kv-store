#!/bin/bash

# Script to deploy infrastructure using Terraform

set -e

MODE=${1:-"leader-follower"}

echo "=========================================="
echo "Deploying Distributed KV Store"
echo "Mode: $MODE"
echo "=========================================="

cd terraform

# Initialize Terraform
echo "Initializing Terraform..."
terraform init -upgrade

# Apply configuration
echo "Applying Terraform configuration..."
terraform apply -auto-approve

echo ""
echo "Deployment complete!"
echo ""

if [ "$MODE" == "leader-follower" ]; then
    echo "Leader URL: http://localhost:8080"
    echo ""
    echo "Test with:"
    echo "  curl -X POST http://localhost:8080/set -H 'Content-Type: application/json' -d '{\"key\":\"test\",\"value\":\"hello\"}'"
    echo "  curl http://localhost:8080/get/test"
else
    echo "Node URLs:"
    echo "  Node 1: http://localhost:8081"
    echo "  Node 2: http://localhost:8082"
    echo "  Node 3: http://localhost:8083"
    echo "  Node 4: http://localhost:8084"
    echo "  Node 5: http://localhost:8085"
    echo ""
    echo "Test with:"
    echo "  curl -X POST http://localhost:8081/set -H 'Content-Type: application/json' -d '{\"key\":\"test\",\"value\":\"hello\"}'"
    echo "  curl http://localhost:8082/get/test"
fi

cd ..

#!/bin/bash

# Script to cleanup infrastructure

set -e

echo "=========================================="
echo "Cleaning up Distributed KV Store"
echo "=========================================="

cd terraform

echo "Destroying infrastructure..."
terraform destroy -auto-approve

echo ""
echo "Cleanup complete!"

cd ..

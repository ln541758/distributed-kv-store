# Distributed Key-Value Database

A distributed key-value database implementation in Go with Leader-Follower and Leaderless architectures, deployed using Terraform.

## Project Structure

```
.
├── leader-follower/          # Leader-Follower implementation
│   ├── main.go              # Entry point
│   ├── kv_store.go          # Core KV store logic
│   ├── server.go            # HTTP server
│   ├── go.mod
│   └── Dockerfile
├── leaderless/              # Leaderless implementation
│   ├── main.go
│   ├── kv_store.go
│   ├── server.go
│   ├── go.mod
│   └── Dockerfile
├── load-tester/             # Load testing tool
│   ├── main.go
│   └── go.mod
├── terraform/               # Infrastructure as Code
│   ├── main.tf
│   ├── leader-follower-w5r1.tf
│   ├── leader-follower-w1r5.tf
│   ├── leader-follower-w3r3.tf
│   ├── leaderless.tf
│   ├── variables.tf
│   └── outputs.tf
├── scripts/                 # Helper scripts
│   ├── deploy.sh
│   ├── cleanup.sh
│   ├── run_leader_follower.sh
│   └── run_leaderless.sh
└── README.md
```

## Prerequisites

- Go 1.21 or higher
- Docker
- Terraform 1.0 or higher

## Quick Start

### Leader-Follower Mode

```bash
# Deploy infrastructure
cd terraform
terraform init
terraform apply

# Wait for services to start
sleep 10

# Test the API
curl -X POST http://localhost:8080/set \
  -H 'Content-Type: application/json' \
  -d '{"key":"test","value":"hello"}'

curl http://localhost:8080/get/test

# Run tests
cd ../tests
go run leader_follower_test.go

# Run load tests
cd ../load-tester
go run main.go -mode leader -write-ratio 0.5 -duration 60 -qps 20

# Cleanup
cd ../terraform
terraform destroy
```

### Leaderless Mode

```bash
# Deploy infrastructure
cd terraform
terraform init
terraform apply

# Wait for services to start
sleep 10

# Test the API (write to any node)
curl -X POST http://localhost:8081/set \
  -H 'Content-Type: application/json' \
  -d '{"key":"test","value":"hello"}'

# Read from any node
curl http://localhost:8082/get/test

# Run tests
cd ../tests
go run leaderless_test.go

# Run load tests
cd ../load-tester
go run main.go -mode leaderless -write-ratio 0.5 -duration 60 -qps 20

# Cleanup
cd ../terraform
terraform destroy
```
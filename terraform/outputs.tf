# Outputs for Terraform configuration

output "leader_url" {
  description = "URL of the leader node (for leader-follower mode)"
  value       = "http://localhost:8080"
}

output "node_urls" {
  description = "URLs of all nodes (for leaderless mode)"
  value = [
    "http://localhost:8081",
    "http://localhost:8082",
    "http://localhost:8083",
    "http://localhost:8084",
    "http://localhost:8085"
  ]
}

output "network_name" {
  description = "Name of the Docker network"
  value       = docker_network.kv_network.name
}

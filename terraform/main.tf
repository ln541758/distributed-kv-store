# Main Terraform configuration for distributed KV store

terraform {
  required_providers {
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 3.0"
    }
  }
}

provider "docker" {
  host = "unix:///var/run/docker.sock"
}

# Network for all containers
resource "docker_network" "kv_network" {
  name = "kv-network"
}

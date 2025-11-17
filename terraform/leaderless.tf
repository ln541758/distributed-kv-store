# Leaderless configuration with W=N, R=1

# Build the leaderless image
resource "docker_image" "leaderless" {
  name = "leaderless:latest"
  build {
    context    = "../leaderless"
    dockerfile = "Dockerfile"
  }
}

# Node 1
resource "docker_container" "node1" {
  name  = "node1"
  image = docker_image.leaderless.image_id

  env = [
    "NODE_ID=node1",
    "W=5",
    "R=1",
    "PORT=8080",
    "PEER_URLS=http://node2:8080,http://node3:8080,http://node4:8080,http://node5:8080"
  ]

  ports {
    internal = 8080
    external = 8081
  }

  networks_advanced {
    name = docker_network.kv_network.name
  }
}

# Node 2
resource "docker_container" "node2" {
  name  = "node2"
  image = docker_image.leaderless.image_id

  env = [
    "NODE_ID=node2",
    "W=5",
    "R=1",
    "PORT=8080",
    "PEER_URLS=http://node1:8080,http://node3:8080,http://node4:8080,http://node5:8080"
  ]

  ports {
    internal = 8080
    external = 8082
  }

  networks_advanced {
    name = docker_network.kv_network.name
  }
}

# Node 3
resource "docker_container" "node3" {
  name  = "node3"
  image = docker_image.leaderless.image_id

  env = [
    "NODE_ID=node3",
    "W=5",
    "R=1",
    "PORT=8080",
    "PEER_URLS=http://node1:8080,http://node2:8080,http://node4:8080,http://node5:8080"
  ]

  ports {
    internal = 8080
    external = 8083
  }

  networks_advanced {
    name = docker_network.kv_network.name
  }
}

# Node 4
resource "docker_container" "node4" {
  name  = "node4"
  image = docker_image.leaderless.image_id

  env = [
    "NODE_ID=node4",
    "W=5",
    "R=1",
    "PORT=8080",
    "PEER_URLS=http://node1:8080,http://node2:8080,http://node3:8080,http://node5:8080"
  ]

  ports {
    internal = 8080
    external = 8084
  }

  networks_advanced {
    name = docker_network.kv_network.name
  }
}

# Node 5
resource "docker_container" "node5" {
  name  = "node5"
  image = docker_image.leaderless.image_id

  env = [
    "NODE_ID=node5",
    "W=5",
    "R=1",
    "PORT=8080",
    "PEER_URLS=http://node1:8080,http://node2:8080,http://node3:8080,http://node4:8080"
  ]

  ports {
    internal = 8080
    external = 8085
  }

  networks_advanced {
    name = docker_network.kv_network.name
  }
}

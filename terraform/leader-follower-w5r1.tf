# Leader-Follower configuration with W=5, R=1

# Build the leader-follower image
resource "docker_image" "leader_follower" {
  name = "leader-follower:latest"
  build {
    context    = "../leader-follower"
    dockerfile = "Dockerfile"
  }
}

# Leader node
resource "docker_container" "leader" {
  name  = "leader"
  image = docker_image.leader_follower.image_id

  env = [
    "NODE_TYPE=leader",
    "W=5",
    "R=1",
    "PORT=8080",
    "FOLLOWER_URLS=http://follower1:8080,http://follower2:8080,http://follower3:8080,http://follower4:8080"
  ]

  ports {
    internal = 8080
    external = 8080
  }

  networks_advanced {
    name = docker_network.kv_network.name
  }
}

# Follower nodes
resource "docker_container" "follower1" {
  name  = "follower1"
  image = docker_image.leader_follower.image_id

  env = [
    "NODE_TYPE=follower",
    "PORT=8080"
  ]

  networks_advanced {
    name = docker_network.kv_network.name
  }
}

resource "docker_container" "follower2" {
  name  = "follower2"
  image = docker_image.leader_follower.image_id

  env = [
    "NODE_TYPE=follower",
    "PORT=8080"
  ]

  networks_advanced {
    name = docker_network.kv_network.name
  }
}

resource "docker_container" "follower3" {
  name  = "follower3"
  image = docker_image.leader_follower.image_id

  env = [
    "NODE_TYPE=follower",
    "PORT=8080"
  ]

  networks_advanced {
    name = docker_network.kv_network.name
  }
}

resource "docker_container" "follower4" {
  name  = "follower4"
  image = docker_image.leader_follower.image_id

  env = [
    "NODE_TYPE=follower",
    "PORT=8080"
  ]

  networks_advanced {
    name = docker_network.kv_network.name
  }
}

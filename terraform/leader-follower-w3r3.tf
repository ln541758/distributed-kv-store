# Leader-Follower configuration with W=3, R=3
# To use this configuration, rename it to override main configuration
# or use terraform workspace

# Uncomment to use W=3, R=3 configuration
/*
resource "docker_container" "leader_w3r3" {
  name  = "leader"
  image = docker_image.leader_follower.image_id

  env = [
    "NODE_TYPE=leader",
    "W=3",
    "R=3",
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
*/

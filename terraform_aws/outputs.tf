output "leader_public_ip" {
  value = aws_instance.leader.public_ip
}

output "follower_public_ips" {
  value = {
    for i, instance in aws_instance.follower :
    "follower-${i + 1}" => instance.public_ip
  }
}
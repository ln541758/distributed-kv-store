# Configuration for AWS provider
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# --- Variables ---
variable "aws_region" {
  description = "The AWS region where resources will be created."
  type        = string
  default     = "us-west-2"
}

/* variable "existing_iam_instance_profile_name" {
  description = "The NAME of an existing IAM Instance Profile that the EC2 instances can use (must have S3 access)."
  type        = string
} */

variable "aws_access_key_id" {
  description = "Your AWS Access Key ID (Sensitive, use only if IAM is blocked)."
  type        = string
  sensitive   = true
}

variable "aws_secret_access_key" {
  description = "Your AWS Secret Access Key (Sensitive, use only if IAM is blocked)."
  type        = string
  sensitive   = true
}

# Provider setup (assumes AWS credentials are configured via environment vars or config file)
provider "aws" {
  region = var.aws_region
}

# --- Dynamic AMI Lookup ---
# Fetch the latest Amazon Linux 2 AMI ID for the specified region
data "aws_ami" "latest_amazon_linux" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["amzn2-ami-hvm-*-x86_64-gp2"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

# --- Instance Profile Lookup
/* data "aws_iam_instance_profile" "selected_profile" {
  instance_profile_name = var.existing_iam_instance_profile_name
} */


# --- Shared Variables and Template Content ---
locals {
  app_name      = "kvstore"
  # ami_id is now dynamically fetched using data.aws_ami.latest_amazon_linux.id
  instance_type = "t2.micro"
  port_app      = 8000
  bucket_name   = "kvstore-test-bucket-${local.app_name}-${random_integer.suffix.result}"
  follower_ports = [8001, 8002, 8003, 8004]

  # Generate the FOLLOWER_URLS string for the Leader (must be done before script generation)
  # NOTE: This join command uses the private_ip attribute of the 'follower' resource, which relies 
  # on the count of the resource. If 'follower' resources haven't been created yet (during plan phase), 
  # this interpolation will correctly reference the future IPs.
  follower_urls = join(",", [
    for i in range(length(aws_instance.follower)) : 
    "http://${aws_instance.follower[i].private_ip}:${local.follower_ports[i]}"
  ])

  # --- RENDERED LEADER SCRIPT ---
  # This script is fully ready, using HCL interpolation to inject variables directly.
  leader_script = <<-EOF
#!/bin/bash
# Script to set up Docker, pull code, and run the kvstore service on EC2.

# 1. Install Docker
sudo yum update -y
sudo yum install -y docker git
sudo systemctl start docker
sudo usermod -a -G docker ec2-user

# Give some time for Docker to initialize
sleep 5

# 2. Clone the Go application code (ASSUMPTION: Replace with your actual repo URL)
# IMPORTANT: You must ensure your Go source code and Dockerfile
# are placed in /home/ec2-user/kvstore_app/leader-follower 
cd /home/ec2-user/
mkdir -p kvstore_app/leader-follower
# --- REPLACE THIS SECTION with actual code cloning/copying ---
# Example: git clone https://github.com/your-user/your-repo.git kvstore_app
# -------------------------------------------------------------

# 3. Build the Docker Image
cd kvstore_app/leader-follower
sudo docker build -t kvstore-app:latest .

# 4. Run the Leader Container
echo "Starting Leader container on port ${local.port_app}"
sudo docker run -d \
    --name leader \
    -p ${local.port_app}:${local.port_app} \
    -e NODE_TYPE="leader" \
    -e PORT=${local.port_app} \
    -e W=5 \
    -e R=1 \
    -e FOLLOWER_URLS="${local.follower_urls}" \
    -e BACKEND_TYPE=s3 \
    -e S3_BUCKET="${aws_s3_bucket.kvstore_bucket.bucket}" \
    -e AWS_ACCESS_KEY_ID="${var.aws_access_key_id}" \
    -e AWS_SECRET_ACCESS_KEY="${var.aws_secret_access_key}" \
    kvstore-app:latest

echo "Deployment finished."
EOF

  # --- FOLLOWER SCRIPT TEMPLATE ---
  # Removed: Template is now directly embedded in the aws_instance.follower resource to avoid templatefile error.
}


# Unique suffix for S3 bucket name
resource "random_integer" "suffix" {
  min = 10000
  max = 99999
}

# --- 1. Networking (VPC and Security Group) ---

resource "aws_vpc" "kvstore_vpc" {
  cidr_block = "10.0.0.0/16"
  tags = { Name = "${local.app_name}-vpc" }
}

resource "aws_subnet" "kvstore_subnet" {
  vpc_id     = aws_vpc.kvstore_vpc.id
  cidr_block = "10.0.1.0/24"
  map_public_ip_on_launch = true
  availability_zone = "${var.aws_region}a" 
  tags = { Name = "${local.app_name}-subnet" }
}

resource "aws_internet_gateway" "gw" {
  vpc_id = aws_vpc.kvstore_vpc.id
  tags = { Name = "${local.app_name}-gw" }
}

resource "aws_route_table" "rt" {
  vpc_id = aws_vpc.kvstore_vpc.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.gw.id
  }
}

resource "aws_route_table_association" "a" {
  subnet_id      = aws_subnet.kvstore_subnet.id
  route_table_id = aws_route_table.rt.id
}

resource "aws_security_group" "kvstore_sg" {
  vpc_id = aws_vpc.kvstore_vpc.id

  # Allow all internal traffic (Leader <-> Follower communication)
  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["10.0.0.0/16"]
  }

  # Allow external SSH (Port 22) and App Access (Port 8000) from your IP
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"] # NOTE: Best practice is to restrict to your IP
  }
  ingress {
    from_port   = local.port_app
    to_port     = local.port_app
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"] # Allow external access to Leader (for testing)
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "tls_private_key" "kv_key" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "aws_key_pair" "kv_keypair" {
  key_name   = "kv-new-key"
  public_key = tls_private_key.kv_key.public_key_openssh
}

resource "local_file" "pem" {
  filename        = "${path.module}/kv-new-key.pem"
  content         = tls_private_key.kv_key.private_key_pem
  file_permission = "0400"
}


# --- 2. S3 and IAM (For S3 Backend Persistence) ---

resource "aws_s3_bucket" "kvstore_bucket" {
  bucket = local.bucket_name
}

/* * The IAM resources were removed. We are now using Access Keys for S3 authentication.
 */

# --- 3. EC2 Instances (Leader and Followers) ---

# Follower Instances (x4) - Uses count
resource "aws_instance" "follower" {
  count                  = length(local.follower_ports)
  # Use the dynamically fetched AMI ID
  ami                    = data.aws_ami.latest_amazon_linux.id
  instance_type          = local.instance_type
  key_name               = "kv-new-key"
  subnet_id              = aws_subnet.kvstore_subnet.id
  vpc_security_group_ids = [aws_security_group.kvstore_sg.id]
  # iam_instance_profile
  
  tags = {
    Name = "${local.app_name}-follower-${count.index + 1}"
  }

  # Inject Follower startup script using direct HCL interpolation
  user_data = base64encode(<<-EOF
#!/bin/bash
# Script to set up Docker, pull code, and run the kvstore service on EC2.

# 1. Install Docker
sudo yum update -y
sudo yum install -y docker git
sudo systemctl start docker
sudo usermod -a -G docker ec2-user

# Give some time for Docker to initialize
sleep 5

# 2. Clone the Go application code (ASSUMPTION: Replace with your actual repo URL)
# IMPORTANT: You must ensure your Go source code and Dockerfile
# are placed in /home/ec2-user/kvstore_app/leader-follower 
cd /home/ec2-user/
mkdir -p kvstore_app/leader-follower
# --- REPLACE THIS SECTION with actual code cloning/copying ---
# Example: git clone https://github.com/your-user/your-repo.git kvstore_app
# -------------------------------------------------------------

# 3. Build the Docker Image
cd kvstore_app/leader-follower
sudo docker build -t kvstore-app:latest .

# 4. Run the Follower Container
echo "Starting Follower container on port ${local.follower_ports[count.index]}"
sudo docker run -d \
    --name follower \
    -p ${local.follower_ports[count.index]}:${local.follower_ports[count.index]} \
    -e NODE_TYPE="follower" \
    -e PORT=${local.follower_ports[count.index]} \
    -e BACKEND_TYPE=s3 \
    -e S3_BUCKET="${aws_s3_bucket.kvstore_bucket.bucket}" \
    -e AWS_ACCESS_KEY_ID="${var.aws_access_key_id}" \
    -e AWS_SECRET_ACCESS_KEY="${var.aws_secret_access_key}" \
    kvstore-app:latest

echo "Deployment finished."
EOF
)
}

# Leader Instance (x1)
resource "aws_instance" "leader" {
  # Use the dynamically fetched AMI ID
  ami                    = data.aws_ami.latest_amazon_linux.id
  instance_type          = local.instance_type
  key_name               = "kv-new-key"
  subnet_id              = aws_subnet.kvstore_subnet.id
  vpc_security_group_ids = [aws_security_group.kvstore_sg.id]
  
  tags = {
    Name = "${local.app_name}-leader"
  }

  # Inject Leader startup script using the fully rendered local script and base64encode
  user_data = base64encode(local.leader_script)
  
  # Ensure the leader is only built after followers are provisioned
  depends_on = [
    aws_instance.follower
  ]
}

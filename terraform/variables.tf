# Variables for Terraform configuration

variable "deployment_mode" {
  description = "Deployment mode: leader-follower or leaderless"
  type        = string
  default     = "leader-follower"

  validation {
    condition     = contains(["leader-follower", "leaderless"], var.deployment_mode)
    error_message = "Deployment mode must be either 'leader-follower' or 'leaderless'."
  }
}

variable "w_value" {
  description = "Write quorum value"
  type        = number
  default     = 5

  validation {
    condition     = var.w_value >= 1 && var.w_value <= 5
    error_message = "W value must be between 1 and 5."
  }
}

variable "r_value" {
  description = "Read quorum value"
  type        = number
  default     = 1

  validation {
    condition     = var.r_value >= 1 && var.r_value <= 5
    error_message = "R value must be between 1 and 5."
  }
}

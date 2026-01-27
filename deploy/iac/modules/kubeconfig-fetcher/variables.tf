variable "host" {
  type        = string
  description = "The IP address or hostname of the remote host"
}

variable "username" {
  type        = string
  description = "The SSH username for authentication"
}

variable "private_key" {
  type        = string
  sensitive   = true
  description = "The SSH private key for authentication"
}

variable "kubeconfig_path" {
  type        = string
  default     = "/etc/kubernetes/admin.conf"
  description = "The path to the kubeconfig file on the remote host"
}

variable "ssh_port" {
  type        = number
  default     = 22
  description = "The SSH port for connection"
}

variable "timeout" {
  type        = string
  default     = "5m"
  description = "The timeout for SSH connection attempts"
}

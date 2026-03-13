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
  default     = "/root/.kube/config"
  description = "The path to the kubeconfig file on the remote host"
}

variable "ssh_port" {
  type        = number
  default     = 22
  description = "The SSH port for connection"
}

variable "ssh_host_public_key" {
  type        = string
  sensitive   = false
  default     = null
  description = "The expected SSH host public key of the remote server. When set, enables strict host key checking to prevent man-in-the-middle attacks."
}

variable "timeout" {
  type        = number
  default     = 300
  description = "The timeout for SSH connection attempts in seconds"
}

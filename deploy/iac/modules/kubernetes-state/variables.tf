variable "kubeconfig_content" {
  type        = string
  sensitive   = true
  description = "The content of the kubeconfig file for Kubernetes cluster authentication"
}

variable "kubeconfig_present" {
  type        = bool
  description = "Tells if the kubeconfig is present"
}

variable "wait_for_deployments" {
  type        = bool
  default     = true
  description = "Whether to wait for all deployments to be ready"
}

variable "deployment_wait_timeout" {
  type        = string
  default     = "5m"
  description = "The timeout to wait for deployments to be ready"
}

variable "namespaces" {
  type        = list(string)
  default     = ["kube-system", "kube-flannel", "kube-public", "default", "kube-node-lease", "local-path-storage"]
  description = "The namespaces to check for deployments"
}

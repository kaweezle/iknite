variable "kubeconfig_content" {
  type        = string
  sensitive   = true
  description = "The content of the kubeconfig file for Kubernetes cluster authentication"
}

variable "kubeconfig_present" {
  type        = bool
  description = "Tells if the kubeconfig is present"
}

variable "releases" {
  type        = map(string)
  description = "Map containing release names and their corresponding helmfile paths"
  default     = {}
}

variable "helmfile_init" {
  type        = bool
  description = "Whether to run helmfile init before applying the helmfile"
  default     = true
}

variable "additional_plugins" {
  type = list(object({
    name    = string
    version = string
    repo    = string
  }))
  description = "List of additional helm plugins to install"
  default = [
    {
      name    = "x"
      version = "0.8.0"
      repo    = "https://github.com/mumoshu/helm-x"
    }
  ]
}

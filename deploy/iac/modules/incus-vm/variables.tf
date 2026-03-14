variable "instances" {
  description = "Incus instances"
  type = map(object({
    name              = string
    image             = string
    description       = string
    type              = optional(string, "container")
    profiles          = list(string)
    ephemeral         = optional(bool, false)
    running           = optional(bool, true)
    config            = optional(map(string))
    project           = optional(string, "default")
    remote            = optional(string)
    ssh_key_name      = optional(string)
    ssh_host_key_name = optional(string)
    wait_for = optional(object({
      type  = optional(string, "ready")
      delay = optional(string)
      nic   = optional(string)
    }))
    devices = optional(map(object({
      name       = string
      type       = string
      properties = map(string)
    })))
    # Files can only be pushed on running instances.
    # files = optional(list(object({
    #   content            = optional(string)
    #   source_path        = optional(string)
    #   target_path        = string
    #   uid                = optional(number)
    #   gid                = optional(number)
    #   mode               = optional(string)
    #   create_directories = optional(bool, false)
    # })))
  }))

  validation {
    condition = alltrue([
      for instance in var.instances : (
        instance.wait_for == null ||
        contains(["agent", "delay", "ipv4", "ipv6", "ready"], coalesce(instance.wait_for.type, "ready"))
      )
    ])
    error_message = "instances[*].wait_for.type must be one of: agent, delay, ipv4, ipv6, ready."
  }
  validation {
    condition = alltrue([
      for instance in var.instances : (
        instance.wait_for == null ||
        contains(["ipv4", "ipv6"], coalesce(instance.wait_for.nic, "ipv4"))
      )
    ])
    error_message = "instances[*].wait_for.nic must be one of: ipv4, ipv6."
  }
  validation {
    condition = alltrue([
      for instance in var.instances : contains(
        ["container", "virtual-machine"],
        coalesce(instance.type, "container")
      )
    ])
    error_message = "instances[*].type must be one of: container, virtual-machine."
  }
  validation {
    condition = alltrue([
      for instance in var.instances : (
        coalesce(instance.type, "container") != "virtual-machine" ||
        trimspace(coalesce(instance.ssh_key_name, "")) != ""
      )
    ])
    error_message = "instances[*].key_name must be set and non-empty when instances[*].type is virtual-machine."
  }
}

variable "ssh_private_keys" {
  type        = map(string)
  description = "Map of private keys for the created keypairs"
  sensitive   = true
}

variable "ssh_host_public_keys" {
  description = "Fixed SSH host keys to configure on instances via cloud-init. When provided, the VM always presents the same host key, enabling strict host key verification without StrictHostKeyChecking=no."
  type        = map(string)
  #   sensitive = true
  default = {}
}

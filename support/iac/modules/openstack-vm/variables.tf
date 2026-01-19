
variable "images" {
  type = map(object({
    name             = string
    local_file_path  = string
    container_format = optional(string, "bare")
    disk_format      = optional(string, "qcow2")
    visibility       = optional(string, "private")
  }))
  description = "Map of images to create"
}

variable "instances" {
  type = map(object({
    name         = string
    enabled      = optional(bool, true)
    image_name   = string
    flavor_name  = string
    key_name     = string
    dns_zone     = optional(string)
    ttl          = optional(number, 60)
    network_name = optional(string, "Ext-Net")
    metadata     = optional(map(string), {})
    user_data    = optional(string, null)
    tags         = optional(list(string), [])
  }))
  description = "Map of instances to create"
}

variable "keys" {
  type = map(object({
    name       = string
    public_key = string
  }))
  description = "Map of keypairs to create"
}

variable "private_keys" {
  type        = map(string)
  description = "Map of private keys for the created keypairs"
  sensitive   = true
}

variable "ovh" {
  type = object({
    endpoint           = optional(string, "ovh-eu")
    application_key    = string
    application_secret = string
    consumer_key       = string
  })
  description = "OVH credentials"
}


variable "openstack" {
  type = object({
    auth_url            = string
    user_domain_name    = string
    project_domain_name = string
    user_name           = string
    password            = string
    region              = string
    tenant_id           = string
  })
  description = "OpenStack credentials"
}

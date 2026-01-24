

variable "instances" {
  description = "Map of instances to create. image_name references a key in var.images."
  type = map(object({
    dns_zone     = optional(string)
    enabled      = optional(bool, true)
    flavor_name  = string
    image_id     = string
    key_name     = string
    metadata     = optional(map(string), {})
    name         = string
    network_name = optional(string, "Ext-Net")
    tags         = optional(list(string), [])
    ttl          = optional(number, 60)
    user_data    = optional(string, null)
  }))
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

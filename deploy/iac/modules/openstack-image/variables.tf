variable "images" {
  description = "Map of images to create"
  type = map(object({
    container_format = optional(string, "bare")
    disk_format      = optional(string, "qcow2")
    local_file_path  = string
    name             = string
    visibility       = optional(string, "private")
  }))
}

variable "openstack" {
  description = "OpenStack credentials"
  type = object({
    auth_url            = string
    password            = string
    project_domain_name = string
    region              = string
    tenant_id           = string
    user_domain_name    = string
    user_name           = string
  })
}

variable "profiles" {
  description = "List of Incus profiles to create."
  type = map(object({
    name        = string
    description = string
    project     = optional(string)
    remote      = optional(string)
    config      = map(string)
    devices = optional(map(object({
      type       = string
      name       = string
      properties = optional(map(string), {})
    })), {})
  }))

}

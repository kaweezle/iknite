
variable "registration_email" {
  type        = string
  description = "Registration email"
}

variable "dns_challenge_providers" {
  type = map(object({
    provider                     = string
    config                       = optional(map(string), {})
    disable_complete_propagation = optional(bool, false)
    pre_check_delay              = optional(number, 30)

  }))
  description = "DNS provider"
}

variable "certificates" {
  type = map(object({
    dns_names              = list(string)
    common_name            = string
    dns_challenge_provider = string
  }))
  description = "List of certificates to be created"
}

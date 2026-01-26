variable "cloudflare_api_token" {
  description = "Cloudflare API token"
  type        = string
}

variable "email" {
  description = "Cloudflare email"
  type        = string

}

variable "cloudflare_account_id" {
  type        = string
  description = "Cloudflare account ID"
}


variable "name" {
  type        = string
  description = "Name of the DNS zone"
}


variable "records" {
  type = map(object({
    name     = string
    enabled  = optional(bool, true)
    type     = string
    content  = string
    proxied  = optional(bool, false)
    ttl      = optional(number)
    priority = optional(number)
    tags     = optional(set(string))
    comment  = optional(string)
  }))
  description = "records"
  default     = {}
  // cSpell: disable
  validation {
    condition = alltrue([
      for record in var.records : can(index(["A", "AAAA", "CAA", "CNAME", "TXT", "SRV", "LOC", "MX", "NS", "SPF", "CERT", "DNSKEY", "DS", "NAPTR", "SMIMEA", "SSHFP", "TLSA", "URI", "PTR", "HTTPS", "SVCB"], record.type) != -1)
    ])
    error_message = "type must be one A, AAAA, CAA, CNAME, TXT, SRV, LOC, MX, NS, SPF, CERT, DNSKEY, DS, NAPTR, SMIMEA, SSHFP, TLSA, URI, PTR, HTTPS, SVCB"
    // cSpell: enable
  }
}


variable "mx_records" {
  type = map(object({
    name     = string
    ttl      = number
    priority = number
    exchange = string
    tags     = optional(map(any))
  }))
  description = "MX records"
  default     = {}
}

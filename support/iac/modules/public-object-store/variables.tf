
variable "ovh" {
  type = object({
    endpoint           = optional(string, "ovh-eu")
    application_key    = string
    application_secret = string
    consumer_key       = string
  })
}

variable "s3" {
  type = object({
    access_key = string
    secret_key = string
  })
  description = "S3 credentials for the object store"
  sensitive   = true
}


variable "project_id" {
  description = "Project ID for the object store"
  type        = string
}

variable "region" {
  description = "Region for the object store"
  type        = string
  default     = "GRA"
}

variable "object_stores" {
  description = "List of object stores to create"
  type = map(object({
    name                = string
    versioning          = optional(bool, false)
    limit               = optional(number, 1000)
    index_document      = optional(string, "index.html")
    error_document      = optional(string, "error.html")
    static_content_path = optional(string)
  }))
}


variable "s3" {
  type = object({
    region     = string
    access_key = string
    secret_key = string
  })
  description = "S3 credentials for the object store"
  sensitive   = true
}

variable "object_stores" {
  description = "List of object stores to create"
  type = map(object({
    name                = string
    static_content_path = string
    destination_prefix  = string
  }))
}

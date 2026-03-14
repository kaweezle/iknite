variable "images" {
  description = "Map of images to create"
  type = map(object({
    data_path     = string
    metadata_path = string
    name          = string
    description   = optional(string, "")
  }))
}

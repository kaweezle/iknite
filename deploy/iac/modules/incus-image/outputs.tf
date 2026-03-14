output "images" {
  value = { for k, v in incus_image.this : k => {
    resource_id = v.resource_id
    fingerprint = v.fingerprint
    alias       = one(v.alias[*].name)
    description = one(v.alias[*].description)
  } }
  description = "Imported images"
}

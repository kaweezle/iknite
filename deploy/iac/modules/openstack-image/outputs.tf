output "images" {
  description = "Details of images created in OpenStack keyed by input map key."
  value = { for k, v in openstack_images_image_v2.this : k => {
    checksum         = v.checksum
    container_format = v.container_format
    disk_format      = v.disk_format
    id               = v.id
    name             = v.name
    visibility       = v.visibility
  } }
}

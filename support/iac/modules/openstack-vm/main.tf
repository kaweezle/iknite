// Maybe these should be separate modules?
resource "openstack_compute_keypair_v2" "this" {
  for_each   = var.keys
  name       = each.value.name
  public_key = each.value.public_key
}

resource "null_resource" "image_hash" {
  for_each = var.images

  triggers = {
    file_hash = filesha256(each.value.local_file_path)
  }
}

// Create new image from current
resource "openstack_images_image_v2" "this" {
  for_each = var.images

  name             = each.value.name
  local_file_path  = each.value.local_file_path
  container_format = each.value.container_format
  disk_format      = each.value.disk_format
  visibility       = each.value.visibility

  lifecycle {
    replace_triggered_by = [
      null_resource.image_hash[each.key].id
    ]
  }
}

locals {
  enabled_instances = {
    for k, v in var.instances : k => v
    if lookup(v, "enabled", true)
  }
}

// Create instance from the image
resource "openstack_compute_instance_v2" "this" {
  for_each    = local.enabled_instances
  name        = each.value.name
  image_id    = openstack_images_image_v2.this[each.value.image_name].id
  flavor_name = each.value.flavor_name
  key_pair    = each.value.key_name
  user_data   = each.value.user_data
  metadata    = each.value.metadata
  tags        = each.value.tags
  network {
    name = each.value.network_name
  }
}

// Only create DNS mappings for instances that have dns_zone set
locals {
  dns_zones_to_create = {
    for k, v in var.instances : k => v
    if v.dns_zone != null
  }
}

// Create DNS entry. Cannot be used for testing because of propagation times.
resource "ovh_domain_zone_record" "this" {
  for_each  = local.dns_zones_to_create
  zone      = each.value.dns_zone
  subdomain = each.value.name
  fieldtype = "A"
  ttl       = each.value.ttl
  target    = openstack_compute_instance_v2.this[each.key].network[0].fixed_ip_v4
}

// Wait for SSH to be available on resource
resource "null_resource" "wait_ssh" {
  depends_on = [openstack_compute_instance_v2.this]
  for_each   = local.enabled_instances
  provisioner "remote-exec" {
    connection {
      type        = "ssh"
      host        = openstack_compute_instance_v2.this[each.key].network[0].fixed_ip_v4
      user        = "root"
      private_key = var.private_keys[each.value.key_name]
      timeout     = "1m"
    }

    inline = ["echo 'connected!'"]
  }

}

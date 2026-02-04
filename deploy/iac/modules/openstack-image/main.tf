resource "null_resource" "image_hash" {
  for_each = var.images

  triggers = {
    file_hash = filesha256(each.value.local_file_path)
  }
}

resource "openstack_images_image_v2" "this" {
  for_each = var.images

  container_format = each.value.container_format
  disk_format      = each.value.disk_format
  local_file_path  = each.value.local_file_path
  name             = each.value.name
  visibility       = each.value.visibility

  lifecycle {
    replace_triggered_by = [
      null_resource.image_hash[each.key].id,
    ]
  }
}

resource "null_resource" "image_hash" {
  for_each = var.images

  triggers = {
    file_hash = filesha256(each.value.data_path)
  }
}

resource "incus_image" "this" {
  for_each = var.images
  alias {
    name        = each.value.name
    description = each.value.description
  }
  source_file = {
    data_path     = each.value.data_path
    metadata_path = each.value.metadata_path
  }

  lifecycle {
    replace_triggered_by = [
      null_resource.image_hash[each.key].id,
    ]
  }
}

resource "incus_profile" "this" {
  for_each    = var.profiles
  name        = each.value.name
  description = each.value.description
  project     = each.value.project
  remote      = each.value.remote
  config      = each.value.config
  dynamic "device" {
    for_each = lookup(each.value, "devices", {})
    content {
      type       = device.value.type
      name       = device.value.name
      properties = device.value.properties
    }
  }
}

resource "cloudflare_zone" "this" {
  account = {
    id = var.cloudflare_account_id
  }
  name = var.name
  type = "full"
}

locals {
  enabled_records = { for k, v in var.records : k => v if v.enabled }
}


resource "cloudflare_dns_record" "this" {
  for_each = local.enabled_records

  zone_id  = cloudflare_zone.this.id
  type     = each.value.type
  name     = each.value.name
  ttl      = each.value.ttl
  tags     = each.value.tags
  content  = each.value.content
  comment  = each.value.comment
  priority = each.value.priority
  proxied  = each.value.proxied
}


resource "cloudflare_dns_record" "mx" {
  for_each = var.mx_records

  zone_id  = cloudflare_zone.this.id
  type     = "MX"
  name     = each.value.name
  ttl      = each.value.ttl
  tags     = each.value.tags
  priority = each.value.priority
  data = {
    target = each.value.exchange
  }
}

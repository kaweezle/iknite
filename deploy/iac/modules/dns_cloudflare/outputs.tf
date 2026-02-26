output "zone" {
  value = {
    id           = data.cloudflare_zone.this.id
    name         = data.cloudflare_zone.this.name
    account_id   = data.cloudflare_zone.this.account.id
    account_name = data.cloudflare_zone.this.account.name
    name_servers = data.cloudflare_zone.this.name_servers
  }
  description = "The zone object"
}

output "records" {
  value       = cloudflare_dns_record.this
  description = "The record objects"
}

output "mx_records" {
  value       = cloudflare_dns_record.mx
  description = "The MX record objects"
}

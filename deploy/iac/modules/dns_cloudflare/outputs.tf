output "zone" {
  value       = cloudflare_zone.this
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

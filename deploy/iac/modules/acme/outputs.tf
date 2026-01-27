output "reg_private_key" {
  value       = tls_private_key.reg.private_key_pem
  description = "Private key for the ACME registration"
  sensitive   = true
}

# output "cert_private_key" {
#   value       = tls_private_key.cert.private_key_pem
#   description = "Private key for the ACME certificate"
#   sensitive   = true
# }

output "certificates" {
  value       = acme_certificate.this
  description = "ACME certificates created"
  sensitive   = true
}

output "private_keys" {
  value       = { for key, cert in acme_certificate.this : key => cert.private_key_pem }
  description = "TLS certificates private keys"
  sensitive   = true
}

output "issuers" {
  value       = { for key, cert in acme_certificate.this : key => "${cert.certificate_pem}${cert.issuer_pem}" }
  description = "TLS certificates issuers"
}

output "pfx" {
  value       = { for key, cert in acme_certificate.this : key => cert.certificate_p12 }
  description = "TLS certificates PFX"
  sensitive   = true
}

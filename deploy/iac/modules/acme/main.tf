
resource "tls_private_key" "reg" {
  algorithm = "RSA"
}

resource "acme_registration" "reg" {
  account_key_pem = tls_private_key.reg.private_key_pem
  email_address   = var.registration_email
}


# FIXME
# It says that it's better to separate the registration private key from the certificate private key
# but if you do that you don't get the PFX that is easily uploaded to Azure Key Vault
# so I'm going to keep it like this for now
# resource "tls_private_key" "cert" {
#   algorithm = "RSA"
# }

# resource "tls_cert_request" "req" {
#   for_each = var.certificates

#   private_key_pem = tls_private_key.cert.private_key_pem
#   dns_names       = each.value.dns_names

#   subject {
#     common_name = each.value.common_name
#   }
# }

resource "acme_certificate" "this" {
  for_each = var.certificates

  account_key_pem           = acme_registration.reg.account_key_pem
  common_name               = each.value.common_name
  subject_alternative_names = each.value.dns_names

  # certificate_request_pem       = tls_cert_request.req[each.key].cert_request_pem
  min_days_remaining            = 30
  revoke_certificate_on_destroy = false

  recursive_nameservers        = ["8.8.8.8:53"]
  disable_complete_propagation = var.dns_challenge_providers[each.value.dns_challenge_provider].disable_complete_propagation
  pre_check_delay              = var.dns_challenge_providers[each.value.dns_challenge_provider].pre_check_delay

  dns_challenge {
    provider = var.dns_challenge_providers[each.value.dns_challenge_provider].provider
    config   = var.dns_challenge_providers[each.value.dns_challenge_provider].config
  }
}

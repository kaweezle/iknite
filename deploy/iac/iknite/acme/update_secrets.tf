
variable "secret_file" {
  type        = string
  description = "Secret file to update"
}

variable "base_key" {
  type        = string
  description = "Base key in the secret file to update"
}

locals {
  base_key = join("", [for item in split(".", var.base_key) : "[\"${replace(item, "~1", ".")}\"]"])
  secrets = { for k, v in var.certificates : k => {
    crt    = join("\\n", split("\n", acme_certificate.this[k].certificate_pem))
    key    = join("\\n", split("\n", acme_certificate.this[k].private_key_pem))
    suffix = v.dns_names[1]
  } }
}

resource "null_resource" "update_secrets" {
  for_each = var.certificates
  provisioner "local-exec" {
    command     = <<-EOT
sops set '${var.secret_file}' '${local.base_key}["${local.secrets[each.key].suffix}"]["crt"]' '"${local.secrets[each.key].crt}"'
sops set '${var.secret_file}' '${local.base_key}["${local.secrets[each.key].suffix}"]["key"]' '"${local.secrets[each.key].key}"'
EOT
    interpreter = ["/bin/bash", "-c"]
  }

  triggers = {
    certificate_pem = acme_certificate.this[each.key].certificate_pem
    private_key_pem = acme_certificate.this[each.key].private_key_pem
  }

  depends_on = [acme_certificate.this]

}

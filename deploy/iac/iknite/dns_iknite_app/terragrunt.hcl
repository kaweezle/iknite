// cSpell: words entraverif dmarc spf
include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/dns_cloudflare"
}

// Import main variables from the root module
locals {
  domain_suffix         = include.root.locals.domain_suffix
  cloudflare_account_id = include.root.locals.cloudflare_account_id
  dns                   = include.root.locals.secret.cloudflare.dns
}

dependency "vm" {
  config_path = "${get_parent_terragrunt_dir("root")}/iknite-vm"
}

inputs = {
  cloudflare_account_id = local.cloudflare_account_id
  cloudflare_api_token  = local.dns.api_token
  email                 = local.dns.email
  name                  = local.domain_suffix
  records = {
    "e2e" = {
      name    = "e2e"
      enabled = can(dependency.vm.outputs.instances["iknite-vm-instance"].access_ip_v4)
      type    = "A"
      content = try(dependency.vm.outputs.instances["iknite-vm-instance"].access_ip_v4, "")
      ttl     = 60
      proxied = false
    }
    "argocd-e2e" = {
      name    = "argocd-e2e"
      enabled = can(dependency.vm.outputs.instances["iknite-vm-instance"].access_ip_v4)
      type    = "A"
      content = try(dependency.vm.outputs.instances["iknite-vm-instance"].access_ip_v4, "")
      ttl     = 60
      proxied = false
    }
  }
}

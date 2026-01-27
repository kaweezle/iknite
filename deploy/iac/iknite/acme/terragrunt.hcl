
include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}


terraform {
  source = "${get_repo_root()}/deploy/iac/modules/acme"
}

// Import main variables from the root module
locals {
  dns   = include.root.locals.secret.cloudflare.dns
  email = include.root.locals.email
}

inputs = {
  registration_email = local.dns.email

  dns_challenge_providers = {
    cloudflare = {
      provider = "cloudflare"
      config = {
        CF_API_EMAIL     = local.dns.email
        CF_DNS_API_TOKEN = local.dns.api_token
      }
    }
  }
  certificates = {
    "all-iknite-app" = {
      common_name            = "*.iknite.app"
      dns_names              = ["*.iknite.app", "iknite.app"]
      dns_challenge_provider = "cloudflare"
    }
  }

}

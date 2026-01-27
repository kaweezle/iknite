include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/github-configuration"
}

// Import main variables from the root module
locals {
  webhook_secret      = include.root.locals.secret.github.webhook_secret
  ssh_public_key      = include.root.locals.secret.iknite_vm.ssh_public_key
  api_token           = include.root.locals.secret.github.api_token
  github_organization = include.root.locals.github_organization
}

inputs = {
  github_token          = local.api_token
  github_owner          = local.github_organization
  repositories          = ["iknite"]
  organizations         = []
  webhook_url           = "https://argocd-e2e.iknite.app/api/webhook"
  webhook_secret        = local.webhook_secret
  deploy_key_public_key = local.ssh_public_key
  deploy_key_title      = "ArgoCD E2E Deploy Key"
  deploy_key_read_only  = true
  webhook_events        = ["push"]
  webhook_active        = true
  webhook_content_type  = "json"
}

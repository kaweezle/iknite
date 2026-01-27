# Note: GitHub OAuth applications cannot be created via Terraform provider.
# The OAuth app must be created manually at https://github.com/settings/developers
# with the following settings:
#   - Application name: {var.oauth_app_name}
#   - Homepage URL: {var.oauth_app_homepage_url}
#   - Authorization callback URL: {var.oauth_callback_url}
# After creation, store the client_secret in the secrets file.

# Deploy keys for repositories
locals {
  # Create a map of repository deploy keys for easier management
  repository_deploy_keys = {
    for repo in var.repositories : repo => {
      title     = var.deploy_key_title
      key       = var.deploy_key_public_key
      read_only = var.deploy_key_read_only
    }
  }
}

resource "github_repository_deploy_key" "this" {
  for_each = local.repository_deploy_keys

  repository = each.key
  title      = each.value.title
  key        = each.value.key
  read_only  = each.value.read_only
}

# Repository webhooks
locals {
  # Create a map of repository webhooks
  repository_webhooks = {
    for repo in var.repositories : repo => {
      url          = var.webhook_url
      secret       = var.webhook_secret
      events       = var.webhook_events
      active       = var.webhook_active
      content_type = var.webhook_content_type
    }
  }
}

resource "github_repository_webhook" "this" {
  for_each = local.repository_webhooks

  repository = each.key
  active     = each.value.active
  events     = each.value.events

  configuration {
    url          = each.value.url
    secret       = each.value.secret
    content_type = each.value.content_type
    insecure_ssl = false
  }
}

# Organization webhooks
locals {
  # Create a map of organization webhooks
  organization_webhooks = {
    for org in var.organizations : org => {
      url          = var.webhook_url
      secret       = var.webhook_secret
      events       = var.webhook_events
      active       = var.webhook_active
      content_type = var.webhook_content_type
    }
  }
}

resource "github_organization_webhook" "this" {
  for_each = local.organization_webhooks

  active = each.value.active
  events = each.value.events

  configuration {
    url          = each.value.url
    secret       = each.value.secret
    content_type = each.value.content_type
    insecure_ssl = false
  }
}

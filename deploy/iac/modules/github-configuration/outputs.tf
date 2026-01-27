output "deploy_keys" {
  value = {
    for k, v in github_repository_deploy_key.this : k => {
      id         = v.id
      title      = v.title
      repository = v.repository
    }
  }
  description = "Deploy key details for each repository"
}

output "repository_webhooks" {
  value = {
    for k, v in github_repository_webhook.this : k => {
      id         = v.id
      url        = v.url
      repository = v.repository
      active     = v.active
      events     = v.events
    }
  }
  description = "Repository webhook details"
}

output "organization_webhooks" {
  value = {
    for k, v in github_organization_webhook.this : k => {
      id     = v.id
      url    = v.url
      active = v.active
      events = v.events
    }
  }
  description = "Organization webhook details"
}

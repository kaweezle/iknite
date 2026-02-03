# GitHub Configuration Module Implementation Summary

<!-- cSpell: words AAAAB -->

## Overview

A comprehensive Terraform module for automating GitHub configuration required by
ArgoCD integration in the Iknite E2E testing infrastructure.

**Module Location**: `/root/iknite/deploy/iac/modules/github-configuration`
**Terragrunt Unit Location**:
`/root/iknite/deploy/iac/iknite/github-configuration`

## Architecture Decision

The module automates all GitHub configurations that can be created via Terraform
while documenting manual steps for OAuth application setup (GitHub provider does
not support OAuth app creation due to one-time secret display limitation).

### What Gets Automated

1. **Deploy Keys** - SSH keys for git repository access by ArgoCD
2. **Repository Webhooks** - Push and pull request webhooks on individual
   repositories
3. **Organization Webhooks** - Organization-level webhooks for broader coverage

### What Requires Manual Setup

1. **OAuth Application** - Must be created manually in GitHub UI with captured
   credentials stored in secrets

## Module Structure

```
deploy/iac/modules/github-configuration/
├── versions.tf           # Terraform and provider version requirements
├── providers.tf          # GitHub provider configuration
├── variables.tf          # Input variables (15 total)
├── main.tf              # Resource definitions (3 resource types)
├── outputs.tf           # Output values (3 outputs)
└── README.md            # Comprehensive documentation
```

## Resources Managed

### Resource 1: Repository Deploy Keys

**Resource Type**: `github_repository_deploy_key` **Created By**: Module for
each repository in `repositories` list

```terraform
resource "github_repository_deploy_key" "this" {
  for_each   = local.repository_deploy_keys
  repository = each.key
  title      = each.value.title
  key        = each.value.key
  read_only  = each.value.read_only
}
```

**Configuration**:

- One deploy key per repository
- Read-only access by default (security best practice)
- SSH public key from `deploy_key_public_key` variable

### Resource 2: Repository Webhooks

**Resource Type**: `github_repository_webhook` **Created By**: Module for each
repository in `repositories` list

```terraform
resource "github_repository_webhook" "this" {
  for_each   = local.repository_webhooks
  repository = each.key
  active     = each.value.active
  events     = each.value.events

  configuration {
    url          = each.value.url
    secret       = each.value.secret
    content_type = each.value.content_type
    insecure_ssl = each.value.insecure_ssl
  }
}
```

**Configuration**:

- Triggered on: `push`, `pull_request` (configurable)
- Content type: JSON
- URL: ArgoCD webhook endpoint (from `webhook_url` variable)
- Secret: Used for webhook authenticity verification

### Resource 3: Organization Webhooks

**Resource Type**: `github_organization_webhook` **Created By**: Module for each
organization in `organizations` list

```terraform
resource "github_organization_webhook" "this" {
  for_each = local.organization_webhooks
  active   = each.value.active
  events   = each.value.events

  configuration {
    url          = each.value.url
    secret       = each.value.secret
    content_type = each.value.content_type
    insecure_ssl = each.value.insecure_ssl
  }
}
```

**Configuration**:

- Triggered on: `push`, `pull_request` (configurable)
- Content type: JSON
- URL: ArgoCD webhook endpoint
- Secret: Webhook authenticity verification

## Input Variables (15 Total)

### Required Variables

1. **github_token** (sensitive)
   - GitHub personal access token with appropriate permissions
   - Used for provider authentication

2. **github_owner** (string)
   - GitHub organization or user owning the resources
   - Example: `"kaweezle"`

3. **deploy_key_public_key** (string)
   - SSH public key for deploy keys
   - Example: `"ssh-rsa AAAAB3NzaC..."`

4. **oauth_callback_url** (string)
   - OAuth callback URL for ArgoCD
   - Example: `"https://argocd-e2e.iknite.app/api/dex/callback"`

5. **oauth_app_homepage_url** (string)
   - Homepage URL for OAuth app
   - Example: `"https://argocd-e2e.iknite.app"`

6. **webhook_url** (string)
   - Webhook endpoint URL in ArgoCD
   - Example: `"https://argocd-e2e.iknite.app/api/webhook"`

7. **webhook_secret** (sensitive string)
   - Secret for securing webhook requests
   - Used to verify webhook authenticity

### Optional Variables with Defaults

1. **repositories** (list(string)) - Default: `[]`
   - List of repository names to configure
   - Example: `["iknite", "kaweezle"]`

2. **organizations** (list(string)) - Default: `[]`
   - List of organization names for webhooks
   - Example: `["kaweezle"]`

3. **deploy_key_title** (string) - Default: `"ArgoCD E2E Deploy Key"`
   - Title/label for the deploy key

4. **deploy_key_read_only** (bool) - Default: `true`
   - Whether deploy key has read-only access (recommended for security)

5. **webhook_active** (bool) - Default: `true`
   - Whether webhooks are enabled

6. **webhook_events** (list(string)) - Default: `["push", "pull_request"]`
   - Events that trigger webhooks

7. **webhook_content_type** (string) - Default: `"json"`
   - Content type for webhook payloads

8. **oauth_app_name** (string) - Default: `"ArgoCD E2E"`
   - Name for the OAuth application (reference only, manual creation)

9. **oauth_permissions** (list(string)) - Default:
   `["read:user", "read:org", "repo"]`
   - OAuth scopes (reference only, manual creation)

## Outputs (3 Total)

### Output 1: deploy_keys

```hcl
output "deploy_keys" {
  description = "Deploy key details for each repository"
  value = {
    for k, v in github_repository_deploy_key.this :
    v.repository => {
      id         = v.id
      repository = v.repository
      title      = v.title
    }
  }
}
```

**Example Output**:

```json
{
  "iknite": {
    "id": "1234567890",
    "repository": "iknite",
    "title": "ArgoCD E2E Deploy Key"
  }
}
```

### Output 2: repository_webhooks

```hcl
output "repository_webhooks" {
  description = "Repository webhook details"
  value = {
    for k, v in github_repository_webhook.this :
    v.repository => {
      id         = v.id
      repository = v.repository
      active     = v.active
      events     = v.events
      url        = v.url
    }
  }
}
```

**Example Output**:

```json
{
  "iknite": {
    "id": "1234567890",
    "repository": "iknite",
    "active": true,
    "events": ["push", "pull_request"],
    "url": "https://github.com/kaweezle/iknite"
  }
}
```

### Output 3: organization_webhooks

```hcl
output "organization_webhooks" {
  description = "Organization webhook details"
  value = {
    for k, v in github_organization_webhook.this :
    k => {
      id     = v.id
      active = v.active
      events = v.events
      url    = v.url
    }
  }
}
```

**Example Output**:

```json
{
  "kaweezle": {
    "id": "1234567890",
    "active": true,
    "events": ["push", "pull_request"],
    "url": "https://github.com/kaweezle"
  }
}
```

## Terragrunt Unit Configuration

**Location**:
`/root/iknite/deploy/iac/iknite/github-configuration/terragrunt.hcl`

```hcl
include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/github-configuration"
}

inputs = {
  github_token          = local.secret.github.api_token
  github_owner          = "kaweezle"
  repositories          = ["iknite"]
  organizations         = ["kaweezle"]
  oauth_callback_url    = "https://argocd-e2e.iknite.app/api/dex/callback"
  oauth_app_homepage_url = "https://argocd-e2e.iknite.app"
  webhook_url           = "https://argocd-e2e.iknite.app/api/webhook"
  webhook_secret        = local.secret.github.webhook_secret
  deploy_key_public_key = local.secret.iknite_vm.ssh_public_key
}
```

**Key Features**:

- Inherits from root.hcl for backend and provider configuration
- Maps inputs from secrets.sops.yaml
- S3 backend state stored at:
  `s3://kwzltfstate/iknite/github-configuration/terraform.tfstate`

## Secrets Integration

The module sources sensitive values from `deploy/iac/iknite/secrets.sops.yaml`:

```yaml
github:
  api_token: ghp_...your_token_here...
  webhook_secret: super_secret_webhook_value

iknite_vm:
  ssh_public_key: ssh-rsa AAAAB3NzaC1yc2E...
```

## GitHub Token Permissions Required

The GitHub token must have the following permissions:

- `admin:org` - For managing organization webhooks
- `admin:repo_hook` - For managing repository webhooks
- `admin:public_key` - For managing deploy keys

## Validation & Testing

### Terragrunt Initialization

```bash
cd /root/iknite/deploy/iac/iknite/github-configuration
terragrunt init
```

**Result**: ✅ SUCCESS

- Backend configured for S3
- GitHub provider v6.10.2 installed
- Terraform initialization complete

### Terragrunt Plan

```bash
terragrunt plan
```

**Result**: ✅ SUCCESS - Plan to create 3 resources:

1. `github_repository_deploy_key.this["iknite"]` - Read-only deploy key
2. `github_repository_webhook.this["iknite"]` - Repository webhook
3. `github_organization_webhook.this["kaweezle"]` - Organization webhook

**Plan Output**:

```
Plan: 3 to add, 0 to change, 0 to destroy.
```

## Manual OAuth Application Setup

Since the GitHub Terraform provider does not support creating OAuth applications
(due to one-time secret display), follow these manual steps:

### Step 1: Navigate to GitHub OAuth Settings

1. Go to GitHub Settings → Developer settings → OAuth Apps
2. Click "New OAuth App"

### Step 2: Fill in Application Details

- **Application name**: "ArgoCD E2E" (from `oauth_app_name`)
- **Homepage URL**: `https://argocd-e2e.iknite.app` (from
  `oauth_app_homepage_url`)
- **Application description**: "ArgoCD OAuth for Iknite E2E Testing"
- **Authorization callback URL**:
  `https://argocd-e2e.iknite.app/api/dex/callback` (from `oauth_callback_url`)

### Step 3: Capture Credentials

After creation, GitHub displays:

- **Client ID**: Public identifier (can be retrieved later)
- **Client Secret**: Secret key (only shown once - **SAVE IMMEDIATELY**)

### Step 4: Store in Secrets

Add to `deploy/iac/iknite/secrets.sops.yaml`:

```yaml
github:
  oauth_client_id: <captured_client_id>
  oauth_client_secret: <captured_client_secret>
```

Encrypt with SOPS:

```bash
sops deploy/iac/iknite/secrets.sops.yaml
```

### Step 5: Configure ArgoCD

Use the stored credentials to configure ArgoCD's OAuth2 provider with GitHub as
the OIDC source.

## Next Steps

1. **Review Module**: Examine the module files to understand the implementation
2. **Adjust Configuration**: Modify `terragrunt.hcl` if needed (repositories,
   organizations, URLs)
3. **Create OAuth App**: Follow manual setup steps above
4. **Apply Configuration**: Run `terragrunt apply` when ready to create
   resources
5. **Configure ArgoCD**: Use OAuth credentials and webhook URLs in ArgoCD
   configuration

## Troubleshooting

### Terraform Plan Fails with "Invalid resource type"

**Issue**: Error mentioning `github_oauth_app` resource not supported
**Solution**: This is expected - OAuth apps cannot be created via Terraform. Use
manual setup steps instead.

### GitHub Token Authorization Fails

**Issue**: Error about insufficient permissions **Solution**: Verify token has
required permissions:

- `admin:org` - Organization webhooks
- `admin:repo_hook` - Repository webhooks
- `admin:public_key` - Deploy keys

### Webhook Secret Doesn't Match

**Issue**: Webhook authenticity verification fails **Solution**: Ensure
`webhook_secret` in terragrunt.hcl matches the value in secrets.sops.yaml

## References

- [GitHub Terraform Provider Documentation](https://registry.terraform.io/providers/integrations/github/latest/docs)
- [GitHub Webhooks Documentation](https://docs.github.com/en/developers/webhooks-and-events/webhooks)
- [ArgoCD OAuth2 OIDC Authentication](https://argo-cd.readthedocs.io/en/stable/operator-manual/user-management/oauth2/#github)
- [Terraform Conventions](./iknite/.github/instructions/terraform.instructions.md)

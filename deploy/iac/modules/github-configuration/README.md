# GitHub Configuration Module

<!-- cSpell: words myorg -->

This module configures GitHub resources required for ArgoCD integration:

- Deploy keys for git repository access (automated)
- Webhooks for repository and organization events (automated)

## How to use

```hcl
module "github_config" {
  source = "../modules/github-configuration"

  github_token         = var.github_token
  github_owner         = "myorg"
  repositories         = ["repo1", "repo2"]
  organizations        = ["myorg"]
  webhook_url          = "https://argocd.example.com/api/webhook"
  webhook_secret       = var.webhook_secret
  deploy_key_public_key = var.ssh_public_key
}
```

## Features

- **Deploy Keys**: Adds SSH deploy keys to repositories for ArgoCD to access
  private repos (automated via `github_repository_deploy_key`)
- **Repository Webhooks**: Configures webhooks on individual repositories to
  trigger ArgoCD syncs (automated via `github_repository_webhook`)
- **Organization Webhooks**: Configures webhooks at the organization level for
  broader coverage (automated via `github_organization_webhook`)

## OAuth Application Setup (Manual)

The GitHub Terraform provider does not support creating OAuth applications
because the client secret is only displayed once at creation. Follow these steps
to manually create the OAuth app:

### Step 1: Navigate to GitHub OAuth Application Settings

1. Go to
   [GitHub Settings > Developer settings > OAuth Apps](https://github.com/settings/developers)
2. Click **"New OAuth App"** button

### Step 2: Register New Application

Fill in the form with the following values:

| Field                      | Value                                                                                            |
| -------------------------- | ------------------------------------------------------------------------------------------------ |
| Application name           | `ArgoCD E2E` (or use `oauth_app_name` variable)                                                  |
| Homepage URL               | Use the value from `oauth_app_homepage_url` (e.g., `https://argocd-e2e.iknite.app`)              |
| Application description    | `ArgoCD OAuth for Iknite E2E Testing`                                                            |
| Authorization callback URL | Use the value from `oauth_callback_url` (e.g., `https://argocd-e2e.iknite.app/api/dex/callback`) |

### Step 3: Capture OAuth Credentials

After creating the application, you will see:

- **Client ID**: A public identifier for the OAuth application
- **Client Secret**: A secret key (only displayed once, save it immediately)

**IMPORTANT**: The client secret is only shown once. Save it to a secure
location (e.g., add to `secrets.sops.yaml`) immediately.

### Step 4: Configure ArgoCD

Use the Client ID and Client Secret obtained above to configure ArgoCD's OAuth2
OIDC provider with GitHub as the authentication source.

## Requirements

### GitHub Token Permissions

The GitHub token must have the following permissions:

- `admin:org` - For managing organization webhooks
- `admin:repo_hook` - For managing repository webhooks
- `admin:public_key` - For managing deploy keys

### SSH Key

The deploy key must be a valid SSH public key (typically generated with
`ssh-keygen`).

## Inputs

See [variables.tf](variables.tf) for detailed input descriptions.

## Outputs

- `deploy_keys` - Deploy key IDs and details for each repository
- `repository_webhooks` - Repository webhook details
- `organization_webhooks` - Organization webhook details

## Notes

- Deploy keys are repository-specific; use read-only mode for security
- Webhooks can be configured at both repository and organization levels
- The webhook secret is used to verify webhook authenticity
- OAuth application must be created manually in GitHub UI (see OAuth Application
  Setup section above)
- Webhook callback URLs must match exactly what is configured in ArgoCD

<!-- markdownlint-disable -->
<!-- BEGIN_TF_DOCS -->
## Requirements

| Name | Version |
|------|---------|
| <a name="requirement_terraform"></a> [terraform](#requirement\_terraform) | >= 1.11.0 |
| <a name="requirement_github"></a> [github](#requirement\_github) | ~> 6.0 |

## Modules

No modules.

## Resources

| Name | Type |
|------|------|
| [github_organization_webhook.this](https://registry.terraform.io/providers/integrations/github/latest/docs/resources/organization_webhook) | resource |
| [github_repository_deploy_key.this](https://registry.terraform.io/providers/integrations/github/latest/docs/resources/repository_deploy_key) | resource |
| [github_repository_webhook.this](https://registry.terraform.io/providers/integrations/github/latest/docs/resources/repository_webhook) | resource |

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| <a name="input_deploy_key_public_key"></a> [deploy\_key\_public\_key](#input\_deploy\_key\_public\_key) | SSH public key to be used as deploy key for git repositories | `string` | n/a | yes |
| <a name="input_deploy_key_read_only"></a> [deploy\_key\_read\_only](#input\_deploy\_key\_read\_only) | Whether the deploy key should be read-only | `bool` | `true` | no |
| <a name="input_deploy_key_title"></a> [deploy\_key\_title](#input\_deploy\_key\_title) | Title for the deploy key | `string` | `"ArgoCD Deploy Key"` | no |
| <a name="input_github_owner"></a> [github\_owner](#input\_github\_owner) | GitHub organization or user name that owns the resources | `string` | `null` | no |
| <a name="input_github_token"></a> [github\_token](#input\_github\_token) | GitHub personal access token with appropriate permissions | `string` | n/a | yes |
| <a name="input_organizations"></a> [organizations](#input\_organizations) | List of organization names to configure webhooks for | `list(string)` | `[]` | no |
| <a name="input_repositories"></a> [repositories](#input\_repositories) | List of repository names (without owner prefix) to configure deploy keys and webhooks for | `list(string)` | `[]` | no |
| <a name="input_webhook_active"></a> [webhook\_active](#input\_webhook\_active) | Whether the webhook is active | `bool` | `true` | no |
| <a name="input_webhook_content_type"></a> [webhook\_content\_type](#input\_webhook\_content\_type) | Content type for webhook payloads | `string` | `"json"` | no |
| <a name="input_webhook_events"></a> [webhook\_events](#input\_webhook\_events) | List of events that should trigger the webhook | `list(string)` | <pre>[<br/>  "push",<br/>  "pull_request"<br/>]</pre> | no |
| <a name="input_webhook_secret"></a> [webhook\_secret](#input\_webhook\_secret) | Secret for securing webhook requests | `string` | n/a | yes |
| <a name="input_webhook_url"></a> [webhook\_url](#input\_webhook\_url) | Webhook URL for ArgoCD to receive webhook events | `string` | n/a | yes |

## Outputs

| Name | Description |
|------|-------------|
| <a name="output_deploy_keys"></a> [deploy\_keys](#output\_deploy\_keys) | Deploy key details for each repository |
| <a name="output_organization_webhooks"></a> [organization\_webhooks](#output\_organization\_webhooks) | Organization webhook details |
| <a name="output_repository_webhooks"></a> [repository\_webhooks](#output\_repository\_webhooks) | Repository webhook details |
<!-- END_TF_DOCS -->
<!-- markdownlint-enable -->

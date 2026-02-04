<!-- cSpell: words addresspool appstage kbst configmap -->

## Intent

The deployment of ArgoCD in a Kubernetes cluster is linked to github in several
ways:

- Github provides Authentication for ArgoCD users through Oauth2.
- Github webhooks are used to trigger ArgoCD syncs when changes are made to the
  git repositories that ArgoCD is monitoring.
- The git repositories themselves are hosted on Github.
- ArgoCD needs access to the git repositories, which require SSH key.

We want to automate as much as possible the configuration of github through a
dedicated terraform module associated with a terragrunt unit.

## Specification

The terraform module will create the following resources:

- A github Oauth application for ArgoCD.
- A github deploy key for each git repository monitored by ArgoCD.
- A github webhook for each git repository or organization monitored by ArgoCD.

The terragrunt unit will provide the following inputs to the terraform module:

- The token of a github user with sufficient permissions to create Oauth
  applications, deploy keys and webhooks.
- The list of git repositories or organizations monitored by ArgoCD.
- The URL of the webhook receiver in the ArgoCD installation.
- The password to protect the webhook receiver in the ArgoCD installation.
- The callback URL for the Oauth application.
- The list of permissions required by the Oauth application.
- The SSH public key to be used as deploy key for the git repositories.

The terraform module will output the following information:

- The client ID and client secret of the Oauth application.
- The list of deploy key IDs for each git repository.
- The list of webhook IDs for each git repository or organization.

Both the terraform module and the terragrunt unit will be documented to explain
how to use them and what are the required inputs and outputs.

They will follow the instructions provided in
[../../../.github/instructions/terraform.instructions.md](../../../.github/instructions/terraform.instructions.md).

# Implementation

the terraform module will be implemented in
[../../modules/github-configuration](../../modules/github-configuration). The
terragrunt unit will be implemented in
[../github-configuration](../github-configuration).

The terragrunt unit will pass the following parameters to the terraform module:

- github_token: Taken from the sops secret file in the `github.api_token` field.
- repositories: `kaweezle/iknite`
- organizations: `kaweezle`
- oauth_callback_url: `https://argocd-e2e.iknite.app/api/dex/callback`
- webhook_url: `https://argocd-e2e.iknite.app/api/webhook`
- webhook_secret: Taken from the sops secret file in the `argocd.webhook_secret`
  field.
- oauth_permissions: `["read:user", "read:org", "repo"]`
- deploy_key_public_key: Taken from the sops secret file in the
  `iknite_vm.ssh_public_key` field.

# Test Plan

The terraform module will be tested by applying the terragrunt unit with the
following commands:

```bash
cd deploy/iac/iknite/github-configuration
export TF_PLUGIN_CACHE_DIR="$HOME/.cache/terraform/plugin-cache"
terragrunt init
terragrunt plan --out=out.plan
```

The plan will be reviewed to ensure that the expected resources are created. It
WILL NOT be applied automatically to avoid creating resources in the actual
github account.

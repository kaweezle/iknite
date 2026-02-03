# cSpell: words hwzkgzs kwzltfstate knttfstate
generate "backend" {
  path      = "backend.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF

terraform {
    backend "s3" {
      bucket = "${local.state_bucket}"
      key    = "iknite/${path_relative_to_include()}/terraform.tfstate"
      region = "${local.secret.cloudflare.storage.region}"
      # sbg or any activated high performance storage region
      endpoints = {
        s3 = "${local.secret.cloudflare.storage.endpoint}/"
      }
      skip_credentials_validation = true
      skip_metadata_api_check     = true
      skip_region_validation      = true
      skip_requesting_account_id  = true
      skip_s3_checksum            = true
      use_path_style              = true

      access_key                  = "${local.secret.cloudflare.storage.access_key_id}"
      secret_key                  = "${local.secret.cloudflare.storage.secret_access_key}"
    }
}
EOF
}

# Generate the versions of the providers if the module does not define it.
# It's better to define the provider version in the module itself to avoid
# version conflicts and depending on terragrunt to manage the provider versions.
generate "required_providers" {
  path      = "versions.tf"
  if_exists = "skip"
  contents  = <<EOF
terraform {
  required_version = ">= 1.11.0"
  required_providers {
    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "3.0.0"
    }
    ovh = {
      source  = "ovh/ovh"
      version = "2.1.0"
    }
  }
}
EOF
}

generate "provider" {
  path      = "providers.tf"
  if_exists = "skip"
  contents  = <<EOF
provider "openstack" {
  // Configured through the OS_CLOUD variable
  alias = "ovh"
}

provider "ovh" {
  endpoint           = "${local.secret.ovh.ovh.endpoint}"
  application_key    = "${local.secret.ovh.ovh.application_key}"
  application_secret = "${local.secret.ovh.ovh.application_secret}"
  consumer_key       = "${local.secret.ovh.ovh.consumer_key}"
}
EOF
}

locals {
  # Project information
  label               = "Kaweezle"
  domain_prefix       = "kaweezle"
  slug                = "kaweezle"
  domain_suffix       = "iknite.app"
  github_organization = "kaweezle"
  state_bucket        = "kwzltfstate"
  email               = "info@kaweezle.com"

  # Infrastructure information
  secret = yamldecode(sops_decrypt_file("${get_repo_root()}/deploy/k8s/secrets/secrets.sops.yaml")).data

  cloudflare_account_id = "a54f6b2557d54a9bff5eef36482b7fe6"

  kubernetes_version = get_env("KUBERNETES_VERSION", "1.35.0")
  iknite_version     = get_env("IKNITE_VERSION", try(jsondecode(file("${get_repo_root()}/dist/metadata.json")).version, "0.6.1-devel"))
}

terraform {
  before_hook " before_hook " {
    commands = [" validate "]
    execute  = [" tflint "]
  }

}

# cSpell: words hwzkgzs kwzltfstate
generate "backend" {
  path      = "backend.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF

terraform {
    backend "s3" {
      bucket = "kwzltfstate"
      key    = "iknite/${path_relative_to_include()}/terraform.tfstate"
      region = "gra"
      # sbg or any activated high performance storage region
      endpoints = {
        s3 = "https://s3.gra.io.cloud.ovh.net/"
      }
      skip_credentials_validation = true
      skip_region_validation      = true
      skip_requesting_account_id  = true
      skip_s3_checksum            = true
      skip_metadata_api_check     = true

      # Credentials. Please configure your credentials in ~/.aws/credentials
      # or in environment variables.
      # Environment variables Example:
      # export AWS_ACCESS_KEY_ID="s3 user access key"
      # export AWS_SECRET_ACCESS_KEY="s3 user secret key"
      # ~/.aws/credentials Example:
      # [default]
      # aws_access_key_id = "s3 user access key"
      # aws_secret_access_key = "s3 user secret key"
      #
      access_key                  = "${local.s3_access_key_id}"
      secret_key                  = "${local.secret.s3_secret_access_key}"
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
  endpoint           = "${local.ovh.endpoint}"
  application_key    = "${local.ovh.application_key}"
  application_secret = "${local.secret.ovh_application_secret}"
  consumer_key       = "${local.ovh.consumer_key}"
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
  email               = "info@kaweezle.com"

  # Infrastructure information
  secret = yamldecode(sops_decrypt_file(find_in_parent_folders("secrets.sops.yaml")))
  ovh = {
    endpoint        = "ovh-eu"
    application_key = "4KyIjUiRpDo4lpqY"
    consumer_key    = "l6t3g737V7WrZbV09urGaRzlRCLZmTqp"
  }
  ovh_region             = "ovh-eu"
  os_project_id          = "40d343bb6eb54b57af7a367a31cd3898"
  os_storage_region_name = "GRA"
  s3_access_key_id       = "a56154691503423d82fe101b6cbd956e"
  s3_user_id             = "a99acb7ecb2f4120b44eddc0b9fd580b"

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

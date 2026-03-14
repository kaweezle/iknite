# cSpell: words hwzkgzs kwzltfstate knttfstate
generate "backend" {
  path      = "backend.tf"
  if_exists = "overwrite_terragrunt"
  contents  = <<EOF

terraform {
    backend "s3" {
      bucket = "${local.state_bucket}"
      key    = "iknite/${path_relative_to_include()}/terraform.tfstate"
      region = "${local.state_storage.region}"
      # sbg or any activated high performance storage region
      endpoints = {
        s3 = "${local.state_storage.endpoint}/"
      }
      skip_credentials_validation = true
      skip_metadata_api_check     = true
      skip_region_validation      = true
      skip_requesting_account_id  = true
      skip_s3_checksum            = true
      use_path_style              = true

      access_key                  = "${local.state_storage.access_key_id}"
      secret_key                  = "${local.state_storage.secret_access_key}"
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


locals {
  values = yamldecode(file("${get_repo_root()}/values.yaml")).data
  # Project information
  label               = local.values.project.label
  slug                = local.values.project.slug
  domain_suffix       = local.values.domain_suffix
  github_organization = split("/", split(":", local.values.project.git.repoURL)[1])[0]
  state_bucket        = local.values.project.state_bucket
  email               = local.values.project.email

  # Retrieve secrets from the SOPS encrypted file
  secret = yamldecode(sops_decrypt_file("${get_repo_root()}/secrets.sops.yaml")).data
  # Change this to the appropriate storage configuration for your environment
  state_storage = local.secret.cloudflare.storage

  kubernetes_version = get_env("KUBERNETES_VERSION", "1.35.2")
  iknite_version     = get_env("IKNITE_VERSION", try(jsondecode(file("${get_repo_root()}/dist/metadata.json")).version, "0.6.1-devel"))
}

terraform {
  before_hook " before_hook " {
    commands = [" validate "]
    execute  = [" tflint "]
  }

}

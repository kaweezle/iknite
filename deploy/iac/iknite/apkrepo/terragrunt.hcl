# cSpell: words apkrepo kwzl
include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/public-object-store"
}

locals {
  s3        = include.root.locals.secret.s3
  openstack = include.root.locals.secret.openstack
  ovh       = include.root.locals.secret.ovh
}

inputs = {
  ovh = local.ovh
  s3 = {
    access_key = local.s3.access_key_id
    secret_key = local.s3.secret_access_key
  }
  project_id = local.openstack.tenant_id
  region     = local.openstack.storage_region

  object_stores = {
    "apkrepo" = {
      name                = "kwzl-apkrepo"
      versioning          = true
      static_content_path = "${get_terragrunt_dir()}/static"
    }
  }
}

# cSpell: words apkrepo kwzl
include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/public-object-store"
}

locals {
  secret        = include.root.locals.secret
  ovh           = include.root.locals.ovh
  project_id    = include.root.locals.os_project_id
  ovh_region    = include.root.locals.os_storage_region_name
  s3_access_key = include.root.locals.s3_access_key_id

}

inputs = {
  ovh = merge(
    local.ovh,
    {
      application_secret = local.secret.ovh_application_secret
    }
  )
  s3 = {
    access_key = local.s3_access_key
    secret_key = local.secret.s3_secret_access_key
  }
  project_id = local.project_id
  region     = local.ovh_region

  object_stores = {
    "apkrepo" = {
      name                = "kwzl-apkrepo"
      versioning          = true
      static_content_path = "${get_terragrunt_dir()}/static"
    }
  }
}

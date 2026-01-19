# cSpell: words iknitestatic
include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/support/iac/modules/object-store-sync"
}

locals {
  secret        = include.root.locals.secret
  s3_region     = lower(include.root.locals.os_storage_region_name)
  s3_access_key = include.root.locals.s3_access_key_id
  r2            = include.root.locals.secret.cloudflare.storage

}

inputs = {
  s3 = {
    access_key = local.r2.access_key_id
    secret_key = local.r2.secret_access_key
    region     = local.r2.region
    endpoint   = local.r2.endpoint
  }

  object_stores = {
    "apkrepo" = {
      name                = "iknitestatic"
      static_content_path = "${get_repo_root()}/dist/repo"
      destination_prefix  = "release/"
    }
  }
}

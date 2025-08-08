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

}

inputs = {
  s3 = {
    access_key = local.s3_access_key
    secret_key = local.secret.s3_secret_access_key
    region     = local.s3_region
  }

  object_stores = {
    "apkrepo" = {
      name                = "kwzl-apkrepo"
      static_content_path = "${get_repo_root()}/dist/repo"
      destination_prefix  = "test/"
    }
  }
}

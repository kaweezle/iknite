include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/openstack-image"
}

locals {
  iknite_version     = include.root.locals.iknite_version
  kubernetes_version = include.root.locals.kubernetes_version
  secret             = include.root.locals.secret
}

inputs = {
  openstack = local.secret.openstack

  images = {
    "iknite-vm-image" = {
      name            = "iknite-test-vm-image-${local.iknite_version}-${local.kubernetes_version}"
      local_file_path = "${get_repo_root()}/dist/iknite-vm.${local.iknite_version}-${local.kubernetes_version}.qcow2"
    }
  }
}

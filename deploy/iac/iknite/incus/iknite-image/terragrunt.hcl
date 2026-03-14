// cSpell: words virtio
include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/incus-image"
}

locals {
  iknite_version     = include.root.locals.iknite_version
  kubernetes_version = include.root.locals.kubernetes_version
}

inputs = {

  images = {
    "iknite-vm-image" = {
      name          = "iknite-vm/${local.iknite_version}-${local.kubernetes_version}"
      data_path     = "${get_repo_root()}/dist/images/iknite-vm.${local.iknite_version}-${local.kubernetes_version}.qcow2"
      metadata_path = "${get_repo_root()}/dist/images/incus.tar.xz"
    }
  }
}

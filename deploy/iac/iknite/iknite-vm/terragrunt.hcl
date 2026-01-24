# cSpell: words apkrepo kwzl
include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/openstack-vm"
}

dependency "image" {
  config_path = "${get_parent_terragrunt_dir("root")}/iknite-image"
}

locals {
  secret             = include.root.locals.secret
  ovh                = include.root.locals.ovh
  iknite_version     = include.root.locals.iknite_version
  kubernetes_version = include.root.locals.kubernetes_version
}

inputs = {
  ovh = merge(
    local.ovh,
    {
      application_secret = local.secret.ovh_application_secret
    }
  )
  openstack = local.secret.openstack
  keys = {
    "iknite" = {
      name       = "iknite"
      public_key = local.secret.iknite_vm.ssh_public_key
    }
  }
  private_keys = {
    "iknite" = local.secret.iknite_vm.ssh_private_key
  }
  instances = {
    "iknite-vm-instance" = {
      name    = "iknite-vm-instance"
      enabled = tobool(get_env("IKNITE_CREATE_INSTANCE", "false"))

      image_id    = dependency.image.outputs.images["iknite-vm-image"].id
      flavor_name = "b3-16"
      key_name    = "iknite"
      user_data   = tobool(get_env("IKNITE_DEBUG_INSTANCE", "false")) ? file("cloud-config.yaml") : null
    }
  }
}

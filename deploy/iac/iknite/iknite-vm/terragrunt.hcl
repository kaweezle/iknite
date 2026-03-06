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

  mock_outputs = {
    images = {
      "iknite-vm-image" = {
        id = "mock-image-id"
      }
    }
  }
}

locals {
  openstack          = include.root.locals.secret.ovh.openstack
  ovh                = include.root.locals.secret.ovh.ovh
  iknite_vm          = include.root.locals.secret.iknite_vm
  iknite_version     = include.root.locals.iknite_version
  kubernetes_version = include.root.locals.kubernetes_version
}

inputs = {
  ovh       = local.ovh
  openstack = local.openstack
  keys = {
    "iknite" = {
      name       = "iknite"
      public_key = local.iknite_vm.ssh_public_key
    }
  }
  private_keys = {
    "iknite" = local.iknite_vm.ssh_private_key
  }
  # Fixed SSH host keys ensure the VM always presents the same host key across
  # recreations, enabling strict host key verification on SSH clients.
  ssh_host_keys = {
    ed25519_private = local.iknite_vm.ssh_host_ed25519_private
    ed25519_public  = local.iknite_vm.ssh_host_ed25519_public
  }
  instances = {
    "iknite-vm-instance" = {
      name    = "iknite-vm-instance"
      enabled = tobool(get_env("IKNITE_CREATE_INSTANCE", "true"))

      image_id    = try(dependency.image.outputs.images["iknite-vm-image"].id, "mock-image-id")
      flavor_name = "b3-16"
      key_name    = "iknite"
      user_data   = tobool(get_env("IKNITE_DEBUG_INSTANCE", "false")) ? file("cloud-config-debug.yaml") : null
    }
  }
}

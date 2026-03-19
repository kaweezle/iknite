// cSpell: words
include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/incus-vm"
}

dependency "images" {
  config_path = "${get_parent_terragrunt_dir("root")}/incus/iknite-image"

  mock_outputs = {
    images = {
      "iknite-vm-image" = {
        "alias"       = "iknite-vm/0.6.4-devel-1.35.2"
        "description" = ""
        "fingerprint" = "1d17b78f947c2684aac5e8d2c6e1016b86e454df6f965b13c7a0e9f2e845f422"
        "resource_id" = ":1d17b78f947c2684aac5e8d2c6e1016b86e454df6f965b13c7a0e9f2e845f422"
      }
    }
  }
}

dependency "profiles" {
  config_path = "${get_parent_terragrunt_dir("root")}/incus/iknite-profiles"

  mock_outputs = {
    profiles = {
      "iknite-container" = {
        "description" = "Profile for iKNIte containers"
        "name"        = "iknite-container"
        "project"     = "default"
        "remote"      = null
      }
      "iknite-vm" = {
        "description" = "Profile for the iKNIte VM image"
        "name"        = "iknite-vm"
        "project"     = "default"
        "remote"      = null
      }
    }
  }
}

locals {
  iknite_version     = include.root.locals.iknite_version
  kubernetes_version = include.root.locals.kubernetes_version
  ssh_private_key    = include.root.locals.secret.keys.main.private_key
  ssh_host_public    = include.root.locals.secret.iknite_vm.ssh_host_ecdsa_public
}

inputs = {
  ssh_private_keys = {
    "iknite-vm-key" = local.ssh_private_key
  }
  ssh_host_public_keys = {
    "iknite-vm-key" = local.ssh_host_public
  }

  instances = {
    "iknite-vm" = {
      name              = "iknite-vm"
      image             = dependency.images.outputs.images["iknite-vm-image"].alias
      description       = "Iknite VM image"
      type              = "virtual-machine"
      ssh_key_name      = "iknite-vm-key"
      ssh_host_key_name = "iknite-vm-key"
      profiles          = ["default", dependency.profiles.outputs.profiles["iknite-vm"].name]
      wait_for = {
        type = "ipv4"
      }
    }
  }
}

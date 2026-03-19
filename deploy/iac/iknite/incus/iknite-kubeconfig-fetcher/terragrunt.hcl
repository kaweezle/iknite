include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/kubeconfig-fetcher"
}

dependency "vm" {
  config_path = "${get_parent_terragrunt_dir("root")}/incus/iknite-vm"

  mock_outputs = {
    instances = {
      "iknite-vm" = {
        "description" = "Iknite VM image"
        "ipv4"        = "10.253.141.182"
        "ipv6"        = "fd42:86db:d3cd:b7ac:1266:6aff:fe32:ce96"
        "name"        = "iknite-vm"
        "project"     = "default"
        "remote"      = null
        "status"      = "Running"
      }
    }
  }
}

locals {
  iknite_vm = include.root.locals.secret.iknite_vm
  ssh_key   = include.root.locals.secret.keys.main
}

inputs = {
  host                = try(dependency.vm.outputs.instances["iknite-vm"].ipv4, "")
  username            = "root"
  private_key         = local.ssh_key.private_key
  ssh_host_public_key = local.iknite_vm.ssh_host_ecdsa_public
  ssh_port            = 22
  timeout             = 300
}

include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/kubeconfig-fetcher"
}

dependency "vm" {
  config_path = "${get_parent_terragrunt_dir("root")}/iknite-vm"

  mock_outputs = {
    instances = {
      "iknite-vm-instance" = {
        access_ip_v4 = "192.168.1.100"
      }
    }
  }
}

locals {
  iknite_vm = include.root.locals.secret.iknite_vm
}

inputs = {
  host        = try(dependency.vm.outputs.instances["iknite-vm-instance"].access_ip_v4, "")
  username    = "root"
  private_key = local.iknite_vm.ssh_private_key
  ssh_port    = 22
  timeout     = "10m"
}

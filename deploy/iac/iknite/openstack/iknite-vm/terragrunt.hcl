# cSpell: words apkrepo kwzl
include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/openstack-vm"
}

dependency "image" {
  config_path = "${get_parent_terragrunt_dir("root")}/openstack/iknite-image"

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
  ssh_key            = include.root.locals.secret.keys.main
  iknite_vm          = include.root.locals.secret.iknite_vm
  iknite_version     = include.root.locals.iknite_version
  kubernetes_version = include.root.locals.kubernetes_version
  git_url            = include.root.locals.github_repo_url
  git_ref            = include.root.locals.github_repo_ref
}

inputs = {
  ovh       = local.ovh
  openstack = local.openstack
  keys = {
    "iknite" = {
      name       = "iknite"
      public_key = local.ssh_key.public_key
    }
  }
  private_keys = {
    "iknite" = local.ssh_key.private_key
  }
  instances = {
    "iknite-vm-instance" = {
      name    = "iknite-vm-instance"
      enabled = tobool(get_env("IKNITE_CREATE_INSTANCE", "true"))

      image_id    = try(dependency.image.outputs.images["iknite-vm-image"].id, "mock-image-id")
      flavor_name = "b3-16"
      key_name    = "iknite"
      user_data   = <<-EOF
#cloud-config
ssh_keys:
    ecdsa_private: |
        ${indent(8, local.iknite_vm.ssh_host_ecdsa_private)}
    ecdsa_public: "${local.iknite_vm.ssh_host_ecdsa_public}"

write_files:
  - path: /opt/iknite/bootstrap/.env
    owner: "root:root"
    permissions: "0640"
    content: |
        IKNITE_BOOTSTRAP_REPO_URL=${local.git_url}
        IKNITE_BOOTSTRAP_REPO_REF=${local.git_ref}
        IKNITE_BOOTSTRAP_SCRIPT=iknite-bootstrap.sh
        GIT_SSH_COMMAND="ssh -i /workspace/.ssh/id_ed25519"
        SOPS_AGE_SSH_PRIVATE_KEY_FILE="/workspace/.ssh/id_ed25519"
  - path: /opt/iknite/bootstrap/.ssh/id_ed25519
    owner: "root:root"
    permissions: "0600"
    content: |
        ${indent(8, local.ssh_key.private_key)}
EOF
    }
  }
}

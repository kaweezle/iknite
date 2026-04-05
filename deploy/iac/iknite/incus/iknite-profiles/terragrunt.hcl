// cSpell: words syscalls kmsg setxattr conntrack hashsize incusbr
include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/incus-profile"
}

locals {
  public_key         = include.root.locals.secret.keys.main.public_key
  private_key        = include.root.locals.secret.keys.main.private_key
  kubernetes_version = include.root.locals.kubernetes_version
  ssh_host_private   = include.root.locals.secret.iknite_vm.ssh_host_ecdsa_private
  ssh_host_public    = include.root.locals.secret.iknite_vm.ssh_host_ecdsa_public
  iknite_vm          = include.root.locals.secret.iknite_vm
  git_url            = include.root.locals.github_repo_url
  git_ref            = include.root.locals.github_repo_ref
}

inputs = {

  profiles = {
    "iknite-vm" = {
      name        = "iknite-vm"
      description = "Profile for the iknite VM image"
      config = {
        "limits.cpu"           = "2"
        "limits.memory"        = "8GB"
        "cloud-init.user-data" = <<-EOF
#cloud-config

ssh_authorized_keys:
  - "${local.public_key}"

ssh_keys:
    ecdsa_private: |
        ${indent(8, local.ssh_host_private)}
    ecdsa_public: "${local.ssh_host_public}"

write_files:
  - path: /etc/iknite.d/iknite.yaml
    owner: "root:root"
    permissions: "0640"
    defer: true
    content: |
        cluster:
            create_ip: false
            kubernetes_version: "${local.kubernetes_version}"
            domain_name: iknite-vm.incus
            network_interface: eth0
            enable_mdns: false
  - path: /opt/iknite/bootstrap/.env
    owner: "root:root"
    permissions: "0640"
    content: |
        KUBEWAIT_BOOTSTRAP_REPO_URL=${local.git_url}
        KUBEWAIT_BOOTSTRAP_REPO_REF=${local.git_ref}
        KUBEWAIT_BOOTSTRAP_SCRIPT=iknite-bootstrap.sh
        GIT_SSH_COMMAND="ssh -i /workspace/.ssh/id_ed25519"
        SOPS_AGE_SSH_PRIVATE_KEY_FILE="/workspace/.ssh/id_ed25519"
  - path: /opt/iknite/bootstrap/.ssh/id_ed25519
    owner: "root:root"
    permissions: "0600"
    content: |
        ${indent(8, local.private_key)}
EOF
      }
      devices = {
        "root" = {
          type = "disk"
          name = "root"
          properties = {
            path = "/"
            pool = "default"
            size = "20GB"
          }
        }
        "cloud-init" = {
          type = "disk"
          name = "agent"
          properties = {
            source = "agent:config"
          }
        }
      }
    }
    "iknite-container" = {
      name        = "iknite-container"
      description = "Profile for iknite containers"
      config = {
        "security.privileged"                     = "true"
        "security.nesting"                        = "true"
        "security.syscalls.intercept.bpf"         = "true"
        "security.syscalls.intercept.bpf.devices" = "true"
        "security.syscalls.intercept.mknod"       = "true"
        "security.syscalls.intercept.setxattr"    = "true"
        "raw.lxc"                                 = <<-EOF
            lxc.apparmor.profile=unconfined
            lxc.sysctl.net.ipv4.ip_forward=1
            lxc.sysctl.net.bridge.bridge-nf-call-iptables=1
            lxc.sysctl.net.bridge.bridge-nf-call-ip6tables=1
            lxc.cgroup2.devices.allow=a
            lxc.mount.auto=proc:rw sys:rw
            lxc.mount.entry = /dev/kmsg dev/kmsg none defaults,bind,create=file
            lxc.hook.start=/root/prepare.sh
        EOF
      }
      devices = {
        "root" = {
          type = "disk"
          name = "root"
          properties = {
            path = "/"
            pool = "default"
          }
        }
        "conntrack_hashsize" = {
          name = "conntrack_hashsize"
          type = "disk"
          properties = {
            path   = "/sys/module/nf_conntrack/parameters/hashsize"
            source = "/sys/module/nf_conntrack/parameters/hashsize"
          }
        }
        "kmsg" = {
          name = "kmsg"
          type = "unix-char"
          properties = {
            path   = "/dev/kmsg"
            source = "/dev/kmsg"
          }
        }
        # Is this needed ?
        "eth0" = {
          name = "eth0"
          type = "nic"
          properties = {
            network = "incusbr0"
          }
        }
      }
    }
  }
}

resource "incus_instance" "this" {
  for_each = var.instances

  name        = each.value.name
  image       = each.value.image
  description = each.value.description
  type        = each.value.type
  profiles    = each.value.profiles
  ephemeral   = each.value.ephemeral
  running     = each.value.running
  config      = each.value.config
  project     = each.value.project
  remote      = each.value.remote

  dynamic "wait_for" {
    for_each = each.value.wait_for == null ? [] : [each.value.wait_for]
    content {
      type  = wait_for.value.type
      delay = wait_for.value.delay
      nic   = wait_for.value.nic
    }
  }

  dynamic "device" {
    for_each = each.value.devices == null ? {} : each.value.devices
    content {
      name       = device.value.name
      type       = device.value.type
      properties = device.value.properties
    }
  }
  # Files can only be pushed on running instances.
  #   dynamic "file" {
  #     for_each = each.value.files == null ? [] : each.value.files
  #     content {
  #       content            = file.value.content
  #       source_path        = file.value.source_path
  #       target_path        = file.value.target_path
  #       uid                = file.value.uid
  #       gid                = file.value.gid
  #       mode               = file.value.mode
  #       create_directories = file.value.create_directories
  #     }
  #   }
}

locals {
  vm_instances = { for k, v in var.instances : k => v if coalesce(v.type, "container") == "virtual-machine" }
}

resource "null_resource" "wait_ssh" {
  depends_on = [incus_instance.this]
  for_each   = local.vm_instances
  provisioner "remote-exec" {
    connection {
      type        = "ssh"
      host        = incus_instance.this[each.key].ipv4_address
      user        = "root"
      private_key = var.ssh_private_keys[each.value.ssh_key_name]
      timeout     = "1m"
      # When fixed SSH host keys are configured, verify the server's host key.
      # This prevents man-in-the-middle attacks during provisioning.
      host_key = var.ssh_host_public_keys != null && each.value.ssh_host_key_name != null ? chomp(var.ssh_host_public_keys[each.value.ssh_host_key_name]) : null

    }

    inline = ["echo 'connected!'"]
  }
}

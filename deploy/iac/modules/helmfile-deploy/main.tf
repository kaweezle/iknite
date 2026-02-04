// cSpell: words crds ireduce
locals {
  kubeconfig_path = "/tmp/kubeconfig-${random_id.config.hex}"
}

# Generate a unique temporary path for kubeconfig
resource "random_id" "config" {
  byte_length = 4
}

# Write kubeconfig to a temporary file
resource "local_file" "kubeconfig" {
  content              = var.kubeconfig_content
  filename             = local.kubeconfig_path
  file_permission      = "0600"
  directory_permission = "0700"
}

# Clean up the temporary kubeconfig file
resource "null_resource" "cleanup_kubeconfig" {
  provisioner "local-exec" {
    when    = destroy
    command = "rm -f ${self.triggers.kubeconfig_file}"
  }

  triggers = {
    kubeconfig_file = local_file.kubeconfig.filename
  }

  depends_on = [local_file.kubeconfig]
}

resource "helmfile_release" "this" {
  for_each     = var.kubeconfig_present ? var.releases : {}
  name         = each.key
  file_or_path = each.value
  kubeconfig   = local_file.kubeconfig.filename
  wait         = true
  destroy = {
    wait = true
  }

  depends_on = [local_file.kubeconfig]
}

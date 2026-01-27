locals {
  # Build SSH connection string for remote command execution
  ssh_command = chomp(<<EOT
eval "$(ssh-agent -s)" > /dev/null && ssh-add <(cat - <<EOF
${var.private_key}
EOF
) > /dev/null && ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=${split("m", var.timeout)[0]} -p ${var.ssh_port} ${var.username}@${var.host} cat ${var.kubeconfig_path}
EOT
  )
}

# Fetch the kubeconfig file from the remote host
resource "null_resource" "fetch_kubeconfig" {
  count = var.host == "" ? 0 : 1

  # This provisional step ensures we only try to fetch once the host is reachable
  provisioner "local-exec" {
    command     = local.ssh_command
    interpreter = ["/bin/bash", "-c"]
    on_failure  = continue
  }

  triggers = {
    host            = var.host
    username        = var.username
    kubeconfig_path = var.kubeconfig_path
    # timestamp       = timestamp()
  }
}

# Retrieve the kubeconfig file content using a data source that runs a script
data "external" "kubeconfig" {
  count   = var.host == "" ? 0 : 1
  program = ["/bin/bash", "-c", "${local.ssh_command} | jq -Rs '{kubeconfig: .}'"]

  depends_on = [null_resource.fetch_kubeconfig]
}

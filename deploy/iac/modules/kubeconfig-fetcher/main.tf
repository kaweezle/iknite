locals {
  # Alpine is using the busybox version of sh which doesn't support parameters to inline scripts.
  # Timeout and kubeconfig cannot be passed as arguments, so we have to embed them directly into the script.
  remote_kubeconfig_script = chomp(<<EOT
:;t="${var.timeout}";e=0;k="${var.kubeconfig_path}"; while [ ! -s "$k" ]; do if [ "$e" -ge "$t" ]; then echo "exit $e $t";exit 1;fi;sleep 1;e=$((e + 1));done;cat "$k"
EOT
  )

  # When a known host key is provided, write it to a temporary file and use
  # StrictHostKeyChecking=yes to prevent man-in-the-middle attacks.
  # Otherwise, fall back to accept-new (accepts new keys but rejects changed ones).
  ssh_command = var.ssh_host_public_key != null ? chomp(<<EOT
eval "$(ssh-agent -s)" > /dev/null && ssh-add <(cat - <<EOF
${var.private_key}
EOF
) > /dev/null && TMP_KH=$(mktemp) && echo "${var.host} ${var.ssh_host_public_key}" > "$TMP_KH" && trap 'rm -f $TMP_KH' EXIT && ssh -o StrictHostKeyChecking=yes -o UserKnownHostsFile="$TMP_KH" -o ConnectTimeout=${var.timeout} -p ${var.ssh_port} ${var.username}@${var.host} sh -c '${local.remote_kubeconfig_script}'
EOT
    ) : chomp(<<EOT
eval "$(ssh-agent -s)" > /dev/null && ssh-add <(cat - <<EOF
${var.private_key}
EOF
) > /dev/null && ssh -o StrictHostKeyChecking=accept-new -o ConnectTimeout=${var.timeout} -p ${var.ssh_port} ${var.username}@${var.host} sh -c '${local.remote_kubeconfig_script}'
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

# Kubeconfig Fetcher Module

This module fetches the kubeconfig file from a remote Kubernetes node via SSH.

## How to use

```hcl
module "kubeconfig_fetcher" {
  source = "../modules/kubeconfig-fetcher"

  host        = "192.168.1.100"
  username    = "root"
  private_key = file("~/.ssh/id_rsa")
  ssh_port    = 22
}
```

## Requirements

- SSH access to the remote host
- `jq` installed on the local machine for JSON processing
- The kubeconfig file must be readable by the SSH user

## Inputs

| Name            | Description                                        | Type     | Default                      | Required |
| --------------- | -------------------------------------------------- | -------- | ---------------------------- | -------- |
| host            | The IP address or hostname of the remote host      | `string` | -                            | yes      |
| username        | The SSH username for authentication                | `string` | -                            | yes      |
| private_key     | The SSH private key for authentication             | `string` | -                            | yes      |
| kubeconfig_path | The path to the kubeconfig file on the remote host | `string` | `/etc/kubernetes/admin.conf` | no       |
| ssh_port        | The SSH port for connection                        | `number` | `22`                         | no       |
| timeout         | The timeout for SSH connection attempts            | `string` | `5m`                         | no       |

## Outputs

| Name       | Description                                                     | Sensitive |
| ---------- | --------------------------------------------------------------- | --------- |
| kubeconfig | The content of the kubeconfig file fetched from the remote host | yes       |

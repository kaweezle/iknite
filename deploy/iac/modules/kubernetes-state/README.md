# Kubernetes State Module

This module retrieves the state of a Kubernetes cluster using a provided
kubeconfig file. It can optionally wait for all deployments to be ready before
completion.

## How to use

```hcl
module "kubernetes_state" {
  source = "../modules/kubernetes-state"

  kubeconfig_content      = module.kubeconfig_fetcher.kubeconfig
  wait_for_deployments    = true
  deployment_wait_timeout = "5m"
}
```

## Requirements

- Valid kubeconfig content with cluster authentication details
- `kubectl` installed on the local machine
- Network access to the Kubernetes cluster API server

## Inputs

| Name                    | Description                                                              | Type           | Default                | Required |
| ----------------------- | ------------------------------------------------------------------------ | -------------- | ---------------------- | -------- |
| kubeconfig_content      | The content of the kubeconfig file for Kubernetes cluster authentication | `string`       | -                      | yes      |
| wait_for_deployments    | Whether to wait for all deployments to be ready                          | `bool`         | `true`                 | no       |
| deployment_wait_timeout | The timeout to wait for deployments to be ready                          | `string`       | `5m`                   | no       |
| namespaces              | The namespaces to check for deployments                                  | `list(string)` | See default namespaces | no       |

## Default Namespaces

- `kube-system` - Kubernetes system components
- `kube-flannel` - Flannel CNI networking
- `kube-public` - Public cluster information
- `default` - Default namespace
- `kube-node-lease` - Node lease information
- `local-path-storage` - Local path provisioner

## Outputs

| Name            | Description                                               | Sensitive |
| --------------- | --------------------------------------------------------- | --------- |
| kubeconfig_path | The path to the temporary kubeconfig file                 | yes       |
| deployments     | The status of all deployments in the cluster by namespace | no        |

## Notes

- The kubeconfig is written to a temporary file with restricted permissions
- The temporary kubeconfig file is cleaned up when the module is destroyed
- The module waits for all deployments in specified namespaces to reach ready
  state before completing

output "kubeconfig" {
  value       = replace(try(data.external.kubeconfig[0].result.kubeconfig, ""), "cluster.iknite", var.host)
  sensitive   = true
  description = "The content of the kubeconfig file fetched from the remote host"
}

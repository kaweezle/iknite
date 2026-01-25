output "kubeconfig" {
  value       = try(data.external.kubeconfig[0].result.kubeconfig, "")
  sensitive   = true
  description = "The content of the kubeconfig file fetched from the remote host"
}

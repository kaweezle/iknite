output "release" {
  value = { for k, v in helmfile_release.this : k => {
    name            = v.name
    sha256_checksum = v.sha256_checksum
    releases_list   = v.releases_list
  } }
  description = "The application result"
}

output "kubeconfig_present" {
  value       = var.kubeconfig_present
  description = "Indicates whether the Kubernetes cluster is accessible"
}

output "kubeconfig_content" {
  value       = var.kubeconfig_content
  sensitive   = true
  description = "The content of the kubeconfig file used for Kubernetes cluster authentication"
}

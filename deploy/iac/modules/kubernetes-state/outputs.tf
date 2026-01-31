output "kubeconfig_present" {
  value       = var.kubeconfig_present
  description = "Indicates whether the Kubernetes cluster is accessible"
}

output "waited_for_resources" {
  value       = var.wait_for_deployments && var.kubeconfig_present
  description = "Indicates whether the module waited for deployments to be ready"
}

output "kubeconfig_content" {
  value       = var.kubeconfig_content
  sensitive   = true
  description = "The content of the kubeconfig file used for Kubernetes cluster authentication"
}

output "deployments" {
  value = { for item in flatten([for ns, deployments in data.kubernetes_resources.deployments : [for obj in deployments.objects : {
    name                 = obj.metadata.name
    namespace            = obj.metadata.namespace
    labels               = obj.metadata.labels
    image                = try(obj.spec.template.spec.containers[0].image, "")
    available_replicas   = try(obj.status.availableReplicas, 0)
    ready_replicas       = try(obj.status.readyReplicas, 0)
    replicas             = try(obj.status.replicas, 0)
    unavailable_replicas = try(obj.status.unavailableReplicas, 0)
  }]]) : "${item.namespace}/${item.name}" => item }
  description = "The status of all deployments in the cluster by namespace"
}

output "daemonsets" {
  value = { for item in flatten([for ns, daemonsets in data.kubernetes_resources.daemonsets : [for obj in daemonsets.objects : {
    name               = obj.metadata.name
    namespace          = obj.metadata.namespace
    labels             = obj.metadata.labels
    image              = try(obj.spec.template.spec.containers[0].image, "")
    available_replicas = try(obj.status.availableReplicas, 0)
    desired_replicas   = try(obj.status.desiredNumberScheduled, 0)
    current_replicas   = try(obj.status.currentNumberScheduled, 0)
  }]]) : "${item.namespace}/${item.name}" => item }
  description = "The status of all daemonsets in the cluster by namespace"
}

output "statefulsets" {
  value = { for item in flatten([for ns, statefulsets in data.kubernetes_resources.statefulsets : [for obj in statefulsets.objects : {
    name             = obj.metadata.name
    namespace        = obj.metadata.namespace
    labels           = obj.metadata.labels
    image            = try(obj.spec.template.spec.containers[0].image, "")
    ready_replicas   = try(obj.status.readyReplicas, 0)
    replicas         = try(obj.status.replicas, 0)
    current_replicas = try(obj.status.currentReplicas, 0)
  }]]) : "${item.namespace}/${item.name}" => item }
  description = "The status of all statefulsets in the cluster by namespace"
}

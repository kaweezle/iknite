locals {
  kubeconfig_path    = "/tmp/kubeconfig-${random_id.config.hex}"
  kubeconfig_content = var.kubeconfig_present ? yamldecode(var.kubeconfig_content) : {}
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

# Configure the Kubernetes provider with the kubeconfig
# Configuration is managed by provider block which depends on kubeconfig being written
provider "kubernetes" {
  host                   = try(local.kubeconfig_content.clusters[0].cluster.server, null)
  client_certificate     = try(base64decode(lookup(local.kubeconfig_content.users[0].user, "client-certificate-data", "")), null)
  client_key             = try(base64decode(lookup(local.kubeconfig_content.users[0].user, "client-key-data", "")), null)
  cluster_ca_certificate = try(base64decode(lookup(local.kubeconfig_content.clusters[0].cluster, "certificate-authority-data", "")), null)
}

# Data source to retrieve all deployments from specified namespaces
data "kubernetes_resources" "deployments" {
  for_each = var.kubeconfig_present ? toset(var.namespaces) : toset([])

  api_version = "apps/v1"
  kind        = "Deployment"
  namespace   = each.value

  depends_on = [local_file.kubeconfig, null_resource.wait]
}

data "kubernetes_resources" "daemonsets" {
  for_each = var.kubeconfig_present ? toset(var.namespaces) : toset([])

  api_version = "apps/v1"
  kind        = "DaemonSet"
  namespace   = each.value

  depends_on = [local_file.kubeconfig, null_resource.wait]
}

data "kubernetes_resources" "statefulsets" {
  for_each = var.kubeconfig_present ? toset(var.namespaces) : toset([])

  api_version = "apps/v1"
  kind        = "StatefulSet"
  namespace   = each.value

  depends_on = [local_file.kubeconfig, null_resource.wait]
}

# Wait for deployments to be ready if enabled
resource "null_resource" "wait" {
  for_each = var.wait_for_deployments && var.kubeconfig_present ? toset(["daemonsets", "statefulsets", "deployments"]) : toset([])

  provisioner "local-exec" {
    command     = join(" ", concat(["${path.module}/wait-resources.sh", local_file.kubeconfig.filename, var.deployment_wait_timeout, each.key], var.namespaces))
    interpreter = ["/bin/bash", "-c"]
  }

  depends_on = [local_file.kubeconfig]
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

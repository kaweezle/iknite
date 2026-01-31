locals {
  kubeconfig_content = var.kubeconfig_present ? yamldecode(var.kubeconfig_content) : {
    apiVersion        = "v1"
    kind              = "Config"
    clusters          = []
    contexts          = []
    "current-context" = ""
    users             = []
  }
}

provider "kubernetes" {
  host                   = try(local.kubeconfig_content.clusters[0].cluster.server, null)
  client_certificate     = try(base64decode(lookup(local.kubeconfig_content.users[0].user, "client-certificate-data", "")), null)
  client_key             = try(base64decode(lookup(local.kubeconfig_content.users[0].user, "client-key-data", "")), null)
  cluster_ca_certificate = try(base64decode(lookup(local.kubeconfig_content.clusters[0].cluster, "certificate-authority-data", "")), null)
}

provider "helmfile" {
  perform_init       = var.helmfile_init
  additional_plugins = var.additional_plugins
  log_level          = "debug"
}

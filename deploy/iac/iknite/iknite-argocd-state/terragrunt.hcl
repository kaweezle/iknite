include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/kubernetes-state"
}

dependency "argocd" {
  config_path = "${get_parent_terragrunt_dir("root")}/iknite-argocd"

  mock_outputs = {
    kubeconfig_content = "apiVersion: v1\nkind: Config\nclusters: []\ncontexts: []\ncurrent-context: ''\nusers: []"
    kubeconfig_present = true
  }
}

inputs = {
  kubeconfig_content      = dependency.argocd.outputs.kubeconfig_content
  kubeconfig_present      = dependency.argocd.outputs.kubeconfig_present
  wait_for_deployments    = true
  deployment_wait_timeout = "5m"
  namespaces              = ["argocd", "traefik"]
}

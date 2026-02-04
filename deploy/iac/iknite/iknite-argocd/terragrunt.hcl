include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/helmfile-deploy"
}

dependency "kubeconfig" {
  config_path = "${get_parent_terragrunt_dir("root")}/iknite-kubernetes-state"

  mock_outputs = {
    kubeconfig_content = "apiVersion: v1\nkind: Config\nclusters: []\ncontexts: []\ncurrent-context: ''\nusers: []"
    kubeconfig_present = true
  }
}

inputs = {
  kubeconfig_content = dependency.kubeconfig.outputs.kubeconfig_content
  kubeconfig_present = dependency.kubeconfig.outputs.kubeconfig_present
  releases = {
    "argocd" = "${get_repo_root()}/deploy/k8s/components/argocd/helmfile.yaml"
  }
}

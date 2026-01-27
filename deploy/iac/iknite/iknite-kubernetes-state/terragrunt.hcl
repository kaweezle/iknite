include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/kubernetes-state"
}

dependency "kubeconfig_fetcher" {
  config_path = "${get_parent_terragrunt_dir("root")}/iknite-kubeconfig-fetcher"

  mock_outputs = {
    kubeconfig = "apiVersion: v1\nkind: Config\nclusters: []\ncontexts: []\ncurrent-context: ''\nusers: []"
  }
}

inputs = {
  kubeconfig_content      = dependency.kubeconfig_fetcher.outputs.kubeconfig
  kubeconfig_present      = dependency.kubeconfig_fetcher.outputs.kubeconfig != ""
  wait_for_deployments    = true
  deployment_wait_timeout = "10m"
  namespaces              = ["kube-system", "kube-flannel", "kube-public", "default", "kube-node-lease", "local-path-storage"]
}

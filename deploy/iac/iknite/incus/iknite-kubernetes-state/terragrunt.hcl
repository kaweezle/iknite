include "root" {
  path   = find_in_parent_folders("root.hcl")
  expose = true
}

terraform {
  source = "${get_repo_root()}/deploy/iac/modules/kubernetes-state"
}

dependency "kubeconfig_fetcher" {
  config_path = "${get_parent_terragrunt_dir("root")}/incus/iknite-kubeconfig-fetcher"

  mock_outputs = {
    kubeconfig = "apiVersion: v1\nkind: Config\nclusters: []\ncontexts: []\ncurrent-context: ''\nusers: []"
  }
}

inputs = {
  kubeconfig_content       = dependency.kubeconfig_fetcher.outputs.kubeconfig
  kubeconfig_present       = dependency.kubeconfig_fetcher.outputs.kubeconfig != ""
  wait_for_deployments     = true
  deployment_wait_timeout  = "10m"
  deployment_settle_period = "5s"
  namespaces               = ["kube-system", "kube-flannel", "kube-public", "default", "local-path-storage", "kgateway-system", "argocd"]
  kubewait_path            = "${get_repo_root()}/dist/kubewait_linux_amd64_v1/kubewait"
}

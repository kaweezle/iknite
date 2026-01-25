<!-- cSpell: words addresspool appstage kbst configmap -->

## Intent

Currently in [release.yml](../../../.github/workflows/release.yml), we test the
produced iknite VM image by provisioning a VM on OpenStack (`iknite-vm`
terragrunt unit using `openstack-vm` module). We wait for SSH to become
available but don't perform any further tests.

We want to add an end-to-end test that involves:

- Waiting for the kubernetes cluster to become ready after bootstrapping
- Deploy ArgoCD onto the cluster via its official Helm chart.
- Make ArgoCD **auto-pilot** itself by pointing it to the iknite Git repository
  and the appropriate path within that repository.
- Make ArgoCD deploy an gateway controller and the resources that expose the
  ArgoCD dashboard.
- Test that the ArgoCD dashboard is reachable.

## Specification

The following modules will be created:

- **Kubeconfig Fetcher Module**: This module will be responsible for fetching
  the kubeconfig file from the provisioned VM. It will handle tasks such as
  connecting to the VM via SSH and retrieving the kubeconfig file. It will be
  named `kubeconfig-fetcher` and located at
  [deploy/iac/modules/kubeconfig-fetcher](../../../deploy/iac/modules/kubeconfig-fetcher/).
  The fetched kubeconfig file content will be made available as an output for
  other modules to use. It will be fetched from `/etc/kubernetes/admin.conf` on
  the VM. The ssh connection details (IP address, username, private key) will be
  provided as input variables to the module.
- **Kubernetes State Module**: This module will be responsible for getting the
  state of the Kubernetes cluster using the fetched kubeconfig file. It will be
  named `kubernetes-state` and located at
  [deploy/iac/modules/kubernetes-state](../../../deploy/iac/modules/kubernetes-state/).
  It will get the status of all the deployments in the cluster and make them
  available as outputs for other modules to use. It will take a boolean
  variables to wait for all deployments to be ready with a timeout.
- **ArgoCD Deployer Module**: This module will be responsible for deploying
  ArgoCD onto the Kubernetes cluster using the fetched kubeconfig file. It will
  use the official ArgoCD Helm chart for deployment and handle tasks such as
  configuring ArgoCD to auto-pilot itself. It will be named `argocd-deployer`
  and located at
  [deploy/iac/modules/argocd-deployer](../../../deploy/iac/modules/argocd-deployer/).
  It will use the kbst/kustomization provider to apply the ArgoCD Helm chart
  along with a job like in
  [kube-vip-addresspool.yaml](../../../packaging/apk/iknite/iknite.d/base/kube-vip-addresspool.yaml)
  that will wait for the ArgoCD deployment to be ready and then create a
  bootstrap argocd application resource pointing to the iknite Git repository.
- **ArgoCD Tester Module**: This module will be responsible for testing the
  reachability of the ArgoCD dashboard. It will handle tasks such as sending
  HTTP requests to the ArgoCD dashboard URL and verifying the response. It will
  be named `argocd-tester` and located at
  [deploy/iac/modules/argocd-tester](../../../deploy/iac/modules/argocd-tester/).

For each module created, a corresponding terragrunt unit will be created under
`deploy/iac/iknite/`:

- **Kubeconfig Fetcher Terragrunt Unit**: This unit will utilize the
  `kubeconfig-fetcher` module to fetch the kubeconfig file from the provisioned
  VM. It will be named `iknite-kubeconfig-fetcher` and located at
  [deploy/iac/iknite/iknite-kubeconfig-fetcher](../../../deploy/iac/iknite/iknite-kubeconfig-fetcher/).
- **Kubernetes State Terragrunt Unit**: This unit will utilize the
  `kubernetes-state` module to get the state of the Kubernetes cluster. It will
  be named `iknite-kubernetes-state` and located at
  [deploy/iac/iknite/iknite-kubernetes-state](../../../deploy/iac/iknite/iknite-kubernetes-state/).
- **ArgoCD Deployer Terragrunt Unit**: This unit will utilize the
  `argocd-deployer` module to deploy ArgoCD onto the Kubernetes cluster. It will
  be named `iknite-argocd-deployer` and located at
  [deploy/iac/iknite/iknite-argocd-deployer](../../../deploy/iac/iknite/iknite-argocd-deployer/).
- **ArgoCD Tester Terragrunt Unit**: This unit will utilize the `argocd-tester`
  module to test the reachability of the ArgoCD dashboard. It will be named
  `iknite-argocd-tester` and located at
  [deploy/iac/iknite/iknite-argocd-tester](../../../deploy/iac/iknite/iknite-argocd-tester/).

A directory `deploy/ops` will be created to hold the kustomizations and ArgoCD
application manifests needed for the ArgoCD deployment and testing. For the
current purpose, all new elements will be created in a subdirectory named
`iknite`. It will be organized as follows:

```
deploy/ops/
└── iknite/
    ├── common/                                         # Common packages, secrets, and values
    │   └── apps/                                       # Common app manifests
    │   │   └── available/                              # Available app manifests
    │   │       ├── argocd.yaml                         # ArgoCD application manifest
    │   │       └── ...                                 # Other app manifests
    │   ├── packages/                                   # Common packages
    │   │   └── argocd/                                 # ArgoCD Kustomization package
    │   │       ├── kustomization.yaml                  # Kustomization for ArgoCD
    │   │       └── ...                                 # Other ArgoCD-related manifests
    │   ├── secrets/                                    # SOPS-encrypted secrets (Kustomization)
    │   └── values/                                     # Common values files (Kustomization)
    ├── e2e/                                            # End-to-end test kustomizations
    │   └── apps/                                       # Common app manifests
    │   │   ├── available/                              # Available app manifests
    │   |   |   └── appstage-00-bootstrap.yaml          # Bootstrap application manifest
    │   │   ├── appstage-00-bootstrap/                  # Bootstrap application kustomization
    │   |   |   ├── appstage-01-online.yaml             # Online stage application manifest
    │   │   |   └── kustomization.yaml                  # Kustomization for bootstrap application
    │   │   ├── appstage-01-online/                     # Online application kustomization
    │   │   |   └── kustomization.yaml                  # Kustomization
    │   ├── packages/                                   # Environment-specific packages
    │   │   └── argocd/                                 # ArgoCD Kustomization package
    │   │       ├── kustomization.yaml                  # Kustomization for ArgoCD
    │   │       └── ...                                 # Other ArgoCD-related manifests
    │   ├── secrets/                                    # SOPS-encrypted secrets (Kustomization)
    │   └── values/                                     # Common values files (Kustomization)
    └── ...                                             # Other environments if needed
```

Kustomizations and application manifests will be created as needed to facilitate
the ArgoCD deployment and testing process.

We assume that kustomize can access files outside its directory, as it is needed
to access the common packages and application manifests from the e2e
kustomization.

The `common` directory will contain shared resources such as the ArgoCD
kustomization package and the ArgoCD application manifest. The `e2e` directory
will contain the kustomizations and manifests specific to the end-to-end testing
process, including the bootstrap application manifest that points to the iknite
Git repository.

The deployment is a multi-stage deployment. A bootstrap application is created
first, which deploys ArgoCD onto the cluster. Since ArgoCD has already been
deployed, this application will _preempt_ the already deployed resources. Then,
an online stage application is created (wave 1), which will deploy additional
resources onto the cluster as needed or launch further stages. This online stage
application will deploy the envoy gateway controller to expose the ArgoCD
dashboard and the appropriate Gateway resources.

The argocd kustomization package under
`deploy/ops/iknite/common/packages/argocd` will define the ArgoCD deployment
using the official Helm chart. It will install it as non-HA. It will be
configured to use github as the OIDC provider for authentication. It will not
deploy ingress resources, as the exposure of the ArgoCD dashboard will be
handled by the online stage application.

The job that injects the bootstrap application resource will be similar to the
one in `kube-vip-addresspool.yaml`, but it will create an ArgoCD application
resource pointing to the iknite Git repository and the appropriate path within
that repository. The application CRD will be defined in a configmap that takes
it data from the
`deploy/ops/iknite/e2e/apps/available/appstage-00-bootstrap.yaml` file.

The domain name for the ArgoCD dashboard will be `argocd-e2e.iknite.app`. We
assume that the DNS for this domain is already configured to point to the
cluster. For now the TLS certificate will be self-signed.

The `appstage-01-online` application is responsible for deploying the following:

- An ingress/gateway controller (e.g., Envoy Gateway) to manage ingress traffic.
- The necessary Gateway resources to expose the ArgoCD dashboard via the
  ingress/gateway controller.

The gateway controller deployment will be implemented in a kustomization package
under `deploy/ops/iknite/common/packages/envoy-gateway-controller`. It will use
the official Envoy Gateway Helm chart for deployment.

## Implementation Steps

1. Implement the `kubeconfig-fetcher` module to fetch the kubeconfig file from
   the provisioned VM.
2. Implement the `iknite-kubeconfig-fetcher` terragrunt unit to utilize the
   `kubeconfig-fetcher` module. It will depend on the existing `iknite-vm` unit
   to get the VM details.
3. Test that the `iknite-kubeconfig-fetcher` unit correctly fetches the
   kubeconfig file from the VM by running:
   ```
   cd deploy/iac/iknite/iknite-image
   terragrunt run --graph --non-interactive plan
   terragrunt run --graph --non-interactive apply
   ```
4. Implement the `kubernetes-state` module to get the state of the Kubernetes
   cluster using the fetched kubeconfig file.
5. Implement the `iknite-kubernetes-state` terragrunt unit to utilize the
   `kubernetes-state` module. It will depend on the `iknite-kubeconfig-fetcher`
   unit to get the kubeconfig file.
6. Test that the `iknite-kubernetes-state` unit correctly gets the state of the
   Kubernetes cluster by running:
   ```
   cd deploy/iac/iknite/iknite-image
   terragrunt run --graph --non-interactive plan
   terragrunt run --graph --non-interactive apply
   ```
7. Implement the `argocd` kustomization package under
   `deploy/ops/iknite/common/packages/argocd` to define the ArgoCD deployment
   using the official Helm chart.
8. Implement the `argocd` application manifest under
   `deploy/ops/iknite/common/apps/available/argocd.yaml` to define the ArgoCD
   application pointing to the iknite Git repository.
9. Implement the `argocd-deployer` module to deploy ArgoCD onto the Kubernetes
   cluster using the fetched kubeconfig file. It will use the
   `kbst/kustomization` provider to apply the
   `deploy/ops/iknite/e2e/packages/argocd` kustomization that uses helm to
   deploy ArgoCD and contains a job resource to create the bootstrap
   application. As terragrunt needs all resources to be in the same directory,
   the `deploy/ops/iknite/` directory will be symlinked into
   `deploy/iac/iknite/iknite-argocd-deployer/ops`. The module will accept as
   input the path to the symlinked `ops` directory.
10. Implement the `iknite-argocd-deployer` terragrunt unit to utilize the
    `argocd-deployer` module. It will depend on the `iknite-kubeconfig-fetcher`
    unit to get the kubeconfig file. The unit directory will contain the symlink
    to the `deploy/ops/iknite/` directory.
11. Implement an empty `appstage-01-online` kustomization under
    `deploy/ops/iknite/e2e/apps/appstage-01-online/` that will be populated in
    the future with resources to expose the ArgoCD dashboard.
12. Test that the `iknite-argocd-deployer` unit correctly deploys ArgoCD by
    running:
    ```
    cd deploy/iac/iknite/iknite-image
    terragrunt run --graph --non-interactive plan
    terragrunt run --graph --non-interactive apply
    ```
13. At this point the bootstrap application should have been created by the job
    and ArgoCD should be deployed onto the cluster. However, as the application
    kustomization is not available on github yet, ArgoCD will not be able to
    sync it.
14. Implement the `envoy-gateway-controller` kustomization package under
    `deploy/ops/iknite/common/packages/envoy-gateway-controller` to define the
    Envoy Gateway controller deployment using the official Helm chart.
15. Implement the necessary Gateway resources under
    `deploy/ops/iknite/e2e/apps/appstage-01-online/` to expose the ArgoCD
    dashboard via the Envoy Gateway controller.
16. Implement the `argocd-tester` module to test the reachability of the ArgoCD
    dashboard using the fetched kubeconfig file.
17. Implement the `iknite-argocd-tester` terragrunt unit to utilize the
    `argocd-tester` module. It will depend on the `iknite-kubeconfig-fetcher`
    unit to get the kubeconfig file.
18. Test that the `iknite-argocd-tester` unit correctly tests the reachability
    of the ArgoCD dashboard by running:
    ```
    cd deploy/iac/iknite/iknite-image
    terragrunt run --graph --non-interactive plan
    terragrunt run --graph --non-interactive apply
    ```

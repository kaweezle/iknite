!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Deploying Argo CD

[Argo CD](https://argo-cd.readthedocs.io/) is a declarative, GitOps continuous
delivery tool for Kubernetes. This tutorial shows how to deploy Argo CD on your
Iknite cluster.

## Prerequisites

- A running Iknite cluster (see [Installation](installation.md))
- `kubectl` configured to access the cluster

## Installation Methods

There are two main approaches to deploy Argo CD on Iknite:

1. **Helmfile method** - Quick manual deployment using helmfile
2. **Bootstrap system** - Automated deployment on cluster startup

## Method 1: Helmfile Installation

The fastest way to deploy Argo CD is using helmfile, which is configured in the
repository:

```bash
helmfile apply -f deploy/k8s/argocd/common/argocd-server/helmfile.yaml.gotmpl
```

Wait for all pods to be ready:

```bash
kubectl wait --for=condition=Available deployment -n argocd --all --timeout=300s
kubectl get pods -n argocd
```

Expose Argo CD via LoadBalancer to access it from outside:

```bash
kubectl patch svc argocd-server -n argocd \
  -p '{"spec": {"type": "LoadBalancer"}}'
```

Get the external IP assigned by Kube-VIP:

```bash
kubectl get svc argocd-server -n argocd
# NAME            TYPE           CLUSTER-IP    EXTERNAL-IP       PORT(S)                      AGE
# argocd-server   LoadBalancer   10.96.5.10    192.168.99.100    80:32123/TCP,443:30456/TCP    1m
```

Retrieve the initial admin password:

```bash
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d
```

Access Argo CD at `https://192.168.99.100` with username `admin`.

!!! warning "TLS certificate" Argo CD uses a self-signed certificate by default.
Accept the security warning in your browser, or add `--insecure` to the Argo CD
server flags.

## Method 2: Bootstrap System

For fully automated deployment on cluster startup, use the Iknite bootstrap
system. This method integrates Argo CD deployment with your custom
infrastructure-as-code workflows.

### How It Works

When Iknite initializes the default kustomization, it launches a bootstrap job
that:

1. **Waits for initial workloads** - Ensures all default components (flannel,
   metrics-server, kgateway, etc.) are ready
2. **Checks for bootstrap configuration** - Looks for configuration in
   `/opt/iknite/bootstrap/`
3. **Clones your repository** - If configured, clones the git repository with
   your infrastructure code
4. **Runs your bootstrap script** - Executes the configured bootstrap script
   (e.g., `iknite-bootstrap.sh`) which can deploy Argo CD and other applications

### Configuring the Bootstrap System

Configure the bootstrap system by placing files in `/opt/iknite/bootstrap/`:

- **`.env`** - Environment variables controlling bootstrap behavior
- **`.ssh/id_ed25519`** - SSH private key for git repository access

#### VM Configuration (via cloud-init)

For VM deployments, configure bootstrap via cloud-init at launch time:

```hcl
# Example from terragrunt configuration
user_data = <<-EOF
#cloud-config
write_files:
  - path: /opt/iknite/bootstrap/.env
    owner: "root:root"
    permissions: "0640"
    content: |
        IKNITE_BOOTSTRAP_REPO_URL=git@github.com:your-org/your-infra.git
        IKNITE_BOOTSTRAP_REPO_REF=main
        IKNITE_BOOTSTRAP_SCRIPT=iknite-bootstrap.sh
  - path: /opt/iknite/bootstrap/.ssh/id_ed25519
    owner: "root:root"
    permissions: "0600"
    content: |
        YOUR_PRIVATE_KEY_HERE
EOF
```

#### Incus / WSL Configuration

For Incus containers or WSL2 environments, mount or create the bootstrap files:

```bash
# Create bootstrap configuration
mkdir -p /opt/iknite/bootstrap/.ssh
cat > /opt/iknite/bootstrap/.env << 'EOF'
IKNITE_BOOTSTRAP_REPO_URL=git@github.com:your-org/your-infra.git
IKNITE_BOOTSTRAP_REPO_REF=main
IKNITE_BOOTSTRAP_SCRIPT=iknite-bootstrap.sh
EOF

# Copy SSH key
cp ~/.ssh/id_ed25519 /opt/iknite/bootstrap/.ssh/id_ed25519
chmod 600 /opt/iknite/bootstrap/.ssh/id_ed25519
```

### Bootstrap Script Example

Create a bootstrap script that deploys Argo CD. Here's the `iknite-bootstrap.sh`
script from the repository as an example:

```bash
#!/bin/bash

# Check that the helmfile command is available
if ! command -v helmfile &> /dev/null; then
  echo "helmfile command not found. Please install helmfile and ensure it is in your PATH."
  exit 1
fi

ROOT_DIR=$(cd "$(dirname "$0")" && pwd)

echo "Applying helmfile for argocd-server"
helmfile apply -f "${ROOT_DIR}/deploy/k8s/argocd/common/argocd-server/helmfile.yaml.gotmpl"
```

You can extend this script with additional infrastructure-as-code steps, such as
configuring repositories, creating applications, or installing other components.

The bootstrap job will:

1. Clone your repository to `/workspace/bootstrap-repo/`
2. Run your `iknite-bootstrap.sh` script
3. Capture all output in logs within `/workspace/logs/`

### Bootstrap Job Details

The bootstrap job is launched automatically after the default kustomization is
applied. See the bootstrap job manifest for advanced configuration.

## Creating Your First Application

Once Argo CD is deployed, create an application to manage your workloads.

### Connect a Git Repository

```bash
argocd repo add https://github.com/your-org/your-repo.git \
  --username your-username \
  --password your-token
```

### Create an Application

Using the CLI:

```bash
argocd app create my-app \
  --repo https://github.com/your-org/your-repo.git \
  --path kubernetes/ \
  --dest-server https://kubernetes.default.svc \
  --dest-namespace default \
  --sync-policy automated
```

Or using a Kubernetes manifest:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/your-org/your-repo.git
    targetRevision: HEAD
    path: kubernetes/
  destination:
    server: https://kubernetes.default.svc
    namespace: default
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

Apply the manifest:

```bash
kubectl apply -f my-app.yaml
```

Sync the application:

```bash
argocd app sync my-app
argocd app status my-app
```

## Installing the Argo CD CLI (Optional)

=== "Windows"

    ```powershell
    scoop install argocd
    ```

=== "macOS"

    ```bash
    brew install argocd
    ```

=== "Linux"

    ```bash
    curl -sSL -o argocd-linux-amd64 \
      https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64
    chmod +x argocd-linux-amd64 && sudo mv argocd-linux-amd64 /usr/local/bin/argocd
    ```

Log in with the CLI:

```bash
argocd login 192.168.99.100 --username admin --password <password> --insecure
```

## Troubleshooting

### Pods stuck in Pending

Check if resources are available:

```bash
kubectl describe pods -n argocd
kubectl top nodes
```

### LoadBalancer IP not assigned

Ensure Kube-VIP is running and the IP pool is configured:

```bash
kubectl get pods -n kube-system | grep kube-vip
kubectl get configmap -n kube-system kube-vip
```

### Bootstrap Job Failures

Check the bootstrap job logs:

```bash
# List bootstrap jobs
kubectl get jobs -n kube-system

# View job logs
kubectl logs -n kube-system job/iknite-bootstrap

# For detailed logs, check the bootstrap workspace
# (accessible if mounted to host)
cat /workspace/logs/rollout_status_*.log
```

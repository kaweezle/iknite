!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Deploying Argo CD

[Argo CD](https://argo-cd.readthedocs.io/) is a declarative, GitOps continuous
delivery tool for Kubernetes. This tutorial shows how to deploy Argo CD on your
Iknite cluster.

## Prerequisites

- A running Iknite cluster (see [Installation](installation.md))
- `kubectl` configured to access the cluster
- A Git repository with Kubernetes manifests (optional for initial setup)

## Installing Argo CD

### Step 1: Create the Namespace

```bash
kubectl create namespace argocd
```

### Step 2: Apply the Argo CD Manifests

```bash
kubectl apply -n argocd -f \
  https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml
```

Wait for all pods to be ready:

```bash
kubectl wait --for=condition=Available deployment -n argocd --all --timeout=300s
kubectl get pods -n argocd
```

### Step 3: Expose Argo CD via LoadBalancer

By default, Argo CD server is a `ClusterIP` service. Expose it as a
`LoadBalancer` to access it from outside:

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

### Step 4: Retrieve the Initial Password

```bash
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d
```

### Step 5: Log In

Open your browser and navigate to `https://192.168.99.100` (the external IP
from the LoadBalancer).

Log in with:
- **Username**: `admin`
- **Password**: (from the previous step)

!!! warning "TLS certificate"
    Argo CD uses a self-signed certificate by default. Accept the security
    warning in your browser, or add `--insecure` to the Argo CD server flags.

### Step 6: Install the Argo CD CLI (Optional)

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

## Creating Your First Application

### Connect a Git Repository

```bash
argocd repo add https://github.com/your-org/your-repo.git \
  --username your-username \
  --password your-token
```

### Create an Application

```bash
argocd app create my-app \
  --repo https://github.com/your-org/your-repo.git \
  --path kubernetes/ \
  --dest-server https://kubernetes.default.svc \
  --dest-namespace default \
  --sync-policy automated
```

Or using a YAML manifest:

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

```bash
kubectl apply -f my-app.yaml
```

### Sync the Application

```bash
argocd app sync my-app
argocd app status my-app
```

## Adding Argo CD to the Bootstrap Kustomization

For automatic Argo CD installation on every cluster start, add it to your
Iknite kustomization:

```yaml
# /etc/iknite.d/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - base/
  - https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

namespace: argocd
```

!!! tip
    Adding Argo CD to the bootstrap kustomization means it will be deployed
    automatically every time the cluster is reset and re-initialized.

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

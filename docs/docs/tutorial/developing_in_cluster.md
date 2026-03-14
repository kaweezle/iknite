<!-- cSpell: words mirrord kwsl -->

!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Developing in the Cluster

Iknite is designed as a development environment. This page explains how to
leverage it for efficient development workflows — from running your code inside
the cluster to using VS Code Remote Containers.

## Development Approaches

| Approach                  | Description                                   | Best For                               |
| ------------------------- | --------------------------------------------- | -------------------------------------- |
| VS Code Remote Containers | Run VS Code connected to a pod                | Full IDE experience inside the cluster |
| mirrord                   | Run local binary as if it were in the cluster | Fast iteration without deploying       |
| Skaffold                  | Auto-build and deploy on code changes         | Automated dev cycle                    |
| BuildKit                  | Build images directly inside the cluster      | Avoid Docker Desktop                   |
| Git hooks                 | Auto-deploy on commit                         | GitOps-style development               |

## VS Code Remote Containers (Dev Containers)

[Dev Containers](https://code.visualstudio.com/docs/devcontainers/containers)
let you run VS Code connected to a container running inside the cluster.

### Prerequisites

- VS Code with
  [Remote - Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)
- `kubectl` configured for your Iknite cluster

### Step 1: Define a Dev Container

Add a `.devcontainer/devcontainer.json` to your project:

```json
{
  "name": "My Dev Container",
  "image": "mcr.microsoft.com/devcontainers/base:ubuntu",
  "features": {
    "ghcr.io/devcontainers/features/kubectl-helm-minikube:1": {},
    "ghcr.io/devcontainers/features/go:1": {}
  },
  "mounts": ["source=${localEnv:HOME}/.kube,target=/root/.kube,type=bind"],
  "postCreateCommand": "go mod download"
}
```

### Step 2: Deploy the Dev Container to the Cluster

```bash
# Build the dev container image using BuildKit
nerdctl build -t my-devcontainer:latest .

# Push to a local registry (optional)
# Or use the image directly from containerd

# Deploy as a pod
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: devcontainer
spec:
  replicas: 1
  selector:
    matchLabels:
      app: devcontainer
  template:
    metadata:
      labels:
        app: devcontainer
    spec:
      containers:
        - name: devcontainer
          image: my-devcontainer:latest
          command: ["sleep", "infinity"]
          volumeMounts:
            - name: workspace
              mountPath: /workspace
      volumes:
        - name: workspace
          hostPath:
            path: /root/workspace
            type: DirectoryOrCreate
EOF
```

### Step 3: Connect VS Code

Press `F1` → **Dev Containers: Attach to Running Container** → select your pod.

## Building Images with BuildKit

Iknite installs BuildKit as part of the default setup. Build images directly
inside the cluster without Docker Desktop:

### Using nerdctl (recommended)

`nerdctl` is the containerd CLI, compatible with Docker syntax:

```bash
# Connect to the Iknite WSL distribution
wsl -d kwsl

# Build an image (inside WSL/VM)
nerdctl build -t my-app:latest ./my-app/

# Run the image
nerdctl run --rm my-app:latest
```

### Using buildctl

```bash
# Build directly with BuildKit
buildctl build \
  --frontend dockerfile.v0 \
  --local context=. \
  --local dockerfile=. \
  --output type=image,name=my-app:latest,push=false
```

### Importing Images into Kubernetes

After building, the image is available in containerd and can be used directly by
Kubernetes pods:

```bash
# Verify the image is available
nerdctl images | grep my-app

# Deploy using the image (no registry needed)
kubectl run my-pod --image=my-app:latest --image-pull-policy=Never
```

## Auto-Deploy with Skaffold

[Skaffold](https://skaffold.dev/) automates the build-push-deploy cycle:

### Install Skaffold

```bash
curl -Lo skaffold https://storage.googleapis.com/skaffold/releases/latest/skaffold-linux-amd64
chmod +x skaffold && sudo mv skaffold /usr/local/bin
```

### Configure Skaffold

```yaml
# skaffold.yaml
apiVersion: skaffold/v4beta11
kind: Config
build:
  artifacts:
    - image: my-app
      context: .
      docker:
        dockerfile: Dockerfile
  local:
    push: false
deploy:
  kubectl:
    manifests:
      - kubernetes/*.yaml
```

### Run Skaffold

```bash
# Develop mode: auto-rebuild and redeploy on changes
skaffold dev
```

## Git Hooks for Auto-Deployment

Use git hooks to automatically deploy changes to the cluster when committing.

### Pre-push Hook

```bash
# .git/hooks/pre-push
#!/bin/bash
set -e

echo "Deploying to Iknite cluster..."
kubectl apply -k kubernetes/
kubectl rollout status deployment/my-app
echo "Deployment successful!"
```

```bash
chmod +x .git/hooks/pre-push
```

### Post-commit Hook with ArgoCD

If using Argo CD, a push to Git triggers automatic sync:

```bash
# .git/hooks/post-commit
#!/bin/bash
# Push to trigger Argo CD sync
git push origin HEAD
argocd app wait my-app --timeout 120
```

## Hot Reloading

For faster iteration, use volume mounts to sync code changes without rebuilding:

```yaml
# kubernetes/deployment.yaml
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: my-app
          image: my-app:dev
          command: ["go", "run", "./..."] # or nodemon, air, etc.
          volumeMounts:
            - name: source-code
              mountPath: /app
      volumes:
        - name: source-code
          hostPath:
            path: /root/workspace/my-app # Path inside WSL/VM
            type: Directory
```

## Environment Variables and Secrets

Inject development credentials securely:

```bash
# Create a secret from a .env file
kubectl create secret generic dev-config --from-env-file=.env

# Reference in deployment
kubectl set env deployment/my-app --from=secret/dev-config
```

## Debugging

### Exec into a Running Pod

```bash
kubectl exec -it deployment/my-app -- /bin/sh
```

### Debug with a Sidecar

```yaml
# Add a debug sidecar
spec:
  containers:
    - name: debugger
      image: alpine:latest
      command: ["sleep", "infinity"]
```

### Use kubectl debug

```bash
# Create an ephemeral debug container
kubectl debug -it deployment/my-app --image=busybox --target=my-app
```

## Tips for Efficient Development

1. **Use mirrord** for the fastest iteration — no build or deploy needed
2. **Pre-pull images** with the `iknite-images` package to avoid delays
3. **Use BuildKit** inside the cluster to avoid Docker Desktop dependency
4. **Use namespaces** to isolate dev, staging, and test environments
5. **Enable Argo CD** for GitOps-based deployment automation

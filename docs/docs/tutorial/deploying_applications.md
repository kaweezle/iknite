!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Deploying Applications

This tutorial explains how to deploy applications on your Iknite cluster using
the **app-of-apps** pattern with Argo CD for GitOps-based management.

## Direct kubectl Deployment

For quick testing, deploy applications directly with `kubectl`:

```bash
# Deploy a simple nginx application
kubectl create deployment nginx --image=nginx:latest
kubectl expose deployment nginx --port=80 --type=LoadBalancer

# Check deployment
kubectl get deployments
kubectl get svc nginx
```

The `LoadBalancer` service type is fully supported by Kube-VIP and gives you
an external IP immediately.

## The App-of-Apps Pattern

The **app-of-apps** pattern is a GitOps strategy where a parent Argo CD
application manages a set of child applications. This creates a Directed Acyclic
Graph (DAG) of applications, where:

1. A **root application** points to a Git repository containing child
   application definitions
2. Each **child application** manages a specific workload in the cluster
3. Changes to the Git repository automatically propagate to the cluster

### Directory Structure

```
my-cluster-apps/
├── root-app.yaml           ← Argo CD root Application pointing to apps/
├── apps/
│   ├── traefik.yaml        ← Traefik ingress controller
│   ├── cert-manager.yaml   ← TLS certificate management
│   ├── monitoring.yaml     ← Prometheus + Grafana stack
│   └── my-app.yaml         ← Your application
└── manifests/
    ├── traefik/            ← Traefik Helm values
    ├── cert-manager/       ← cert-manager configuration
    └── my-app/             ← Application manifests
```

### Step 1: Create the Root Application

```yaml
# root-app.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: root
  namespace: argocd
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: default
  source:
    repoURL: https://github.com/your-org/my-cluster-apps.git
    targetRevision: HEAD
    path: apps/
  destination:
    server: https://kubernetes.default.svc
    namespace: argocd
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

### Step 2: Define Child Applications

```yaml
# apps/traefik.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: traefik
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://helm.traefik.io/traefik
    chart: traefik
    targetRevision: "28.x"
    helm:
      valuesFiles:
        - values.yaml
  destination:
    server: https://kubernetes.default.svc
    namespace: traefik
  syncPolicy:
    automated: {}
    syncOptions:
      - CreateNamespace=true
```

### Step 3: Apply the Root Application

```bash
kubectl apply -f root-app.yaml
```

Argo CD picks up the root application and automatically deploys all child
applications:

```bash
argocd app list
# NAME        CLUSTER                         NAMESPACE  PROJECT  STATUS  HEALTH   SYNCPOLICY  ...
# root        https://kubernetes.default.svc  argocd     default  Synced  Healthy  Auto-Prune  ...
# traefik     https://kubernetes.default.svc  traefik    default  Synced  Healthy  Auto        ...
```

## Deploying with Helm

Iknite's Kubernetes is fully compatible with Helm:

```bash
# Install Helm
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# Add a chart repository
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

# Install a chart
helm install my-wordpress bitnami/wordpress \
  --namespace wordpress \
  --create-namespace \
  --set service.type=LoadBalancer
```

## Deploying with Kustomize

Kustomize is built into `kubectl`:

```bash
# Apply a kustomization
kubectl apply -k ./my-kustomization/

# Preview without applying
kubectl kustomize ./my-kustomization/
```

Iknite also exposes its kustomize wrapper:

```bash
# Print the resolved kustomization
iknite kustomize -d /etc/iknite.d print

# Apply a custom kustomization
iknite kustomize -d ./my-kustomization apply
```

## Using Persistent Storage

Local Path Provisioner creates a default storage class. Use it in your
deployments:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-data
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: local-path
  resources:
    requests:
      storage: 1Gi
```

```bash
kubectl apply -f pvc.yaml
kubectl get pvc my-data
# NAME      STATUS   VOLUME                   CAPACITY   ACCESS MODES   STORAGECLASS   AGE
# my-data   Bound    pvc-xxxxxxxxxxxxxxxxxx   1Gi        RWO            local-path     5s
```

Data is stored at `/opt/local-path-provisioner/<namespace>-<pvc-name>/` on the
host filesystem.

## Ingress with Traefik

With Kube-VIP providing a LoadBalancer IP, Traefik can be deployed as an
ingress controller:

### Install Traefik

```bash
helm repo add traefik https://helm.traefik.io/traefik
helm install traefik traefik/traefik \
  --namespace traefik \
  --create-namespace \
  --set service.type=LoadBalancer
```

### Create an Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app-ingress
  annotations:
    traefik.ingress.kubernetes.io/router.entrypoints: web
spec:
  rules:
    - host: my-app.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-app
                port:
                  number: 80
```

Add the host to your Windows `hosts` file:

```
192.168.99.100 my-app.local
```

## Best Practices for App Management

1. **Use GitOps**: Store all manifests in Git and use Argo CD for deployment
2. **Use namespaces**: Separate applications by namespace for isolation
3. **Resource limits**: Always set CPU/memory limits for production workloads
4. **Health checks**: Add readiness and liveness probes
5. **Use the app-of-apps pattern** for managing multiple applications consistently

## Next Steps

- [Tunneling](tunneling.md) – Access services from outside the cluster
- [Developing in the cluster](developing_in_cluster.md) – Development workflows
- [Best Practices](../user_guide/best_practices.md) – Optimization tips

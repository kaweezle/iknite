!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Kubernetes Configuration Overview

This page provides an overview of the Kubernetes components managed by Iknite
and how Iknite interacts with them.

## Component Stack

```
┌─────────────────────────────────────────────────────┐
│                     iknite                          │
│  ┌────────────────────────────────────────────────┐ │
│  │               kubeadm (embedded)               │ │
│  │  ┌──────────────────────────────────────────┐  │ │
│  │  │          Kubernetes Control Plane         │  │ │
│  │  │  kube-apiserver   kube-scheduler         │  │ │
│  │  │  kube-controller-manager                 │  │ │
│  │  │  kine (or etcd)                          │  │ │
│  │  └──────────────────────────────────────────┘  │ │
│  │  ┌──────────────────────────────────────────┐  │ │
│  │  │               kubelet                    │  │ │
│  │  └──────────────────────────────────────────┘  │ │
│  └────────────────────────────────────────────────┘ │
│  ┌────────────────────────────────────────────────┐ │
│  │               containerd                       │ │
│  │  ┌──────────────────────────────────────────┐  │ │
│  │  │            BuildKit                      │  │ │
│  │  └──────────────────────────────────────────┘  │ │
│  └────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
        Alpine Linux / OpenRC
```

## Components

### kubeadm

[kubeadm](https://kubernetes.io/docs/reference/setup-tools/kubeadm/) is the
official Kubernetes cluster bootstrapping tool. Iknite uses kubeadm for:

- Certificate generation (`/etc/kubernetes/pki/`)
- Control plane manifests generation (`/etc/kubernetes/manifests/`)
- kubeconfig files generation (`/etc/kubernetes/*.conf`)
- kubelet configuration (`/var/lib/kubelet/`)

Iknite does **not** shell out to kubeadm. Instead, it imports kubeadm as a Go
library and uses `//go:linkname` to access and modify its internal phases.

### kubelet

[kubelet](https://kubernetes.io/docs/reference/command-line-tools-reference/kubelet/)
is the node agent that manages pod lifecycle. Iknite manages kubelet as a
**direct subprocess** rather than an OpenRC service, giving it precise control
over the kubelet lifecycle.

Key kubelet configuration files:

| File | Purpose |
|------|---------|
| `/var/lib/kubelet/config.yaml` | Kubelet configuration |
| `/var/lib/kubelet/kubeadm-flags.env` | kubeadm-generated flags |
| `/etc/kubernetes/kubelet.conf` | Kubelet kubeconfig |

### containerd

[containerd](https://containerd.io/) is the container runtime. It is managed
by OpenRC and serves as the CRI (Container Runtime Interface) for Kubernetes.

Key configuration:

| File | Purpose |
|------|---------|
| `/etc/containerd/config.toml` | containerd configuration |
| `/run/containerd/containerd.sock` | containerd socket |
| `/etc/crictl.yaml` | crictl configuration |

### kine (default API backend)

[kine](https://github.com/k3s-io/kine) is a lightweight etcd replacement that
stores Kubernetes API objects in a SQLite database. It is deployed as a static
pod.

| File | Purpose |
|------|---------|
| `/etc/kubernetes/manifests/kine.yaml` | Static pod manifest |
| `/var/lib/kine/kine.db` | SQLite database |

### etcd (optional)

[etcd](https://etcd.io/) is the standard distributed key-value store for
Kubernetes. Enable it with `useEtcd: true` in configuration.

| File | Purpose |
|------|---------|
| `/etc/kubernetes/manifests/etcd.yaml` | Static pod manifest |
| `/var/lib/etcd/` | etcd data directory |

## Static Pods

The Kubernetes control plane runs as **static pods** managed by kubelet:

| Pod | File | Description |
|-----|------|-------------|
| kube-apiserver | `/etc/kubernetes/manifests/kube-apiserver.yaml` | API server |
| kube-controller-manager | `/etc/kubernetes/manifests/kube-controller-manager.yaml` | Controller manager |
| kube-scheduler | `/etc/kubernetes/manifests/kube-scheduler.yaml` | Scheduler |
| kube-vip | `/etc/kubernetes/manifests/kube-vip.yaml` | VIP / load balancer |
| kine | `/etc/kubernetes/manifests/kine.yaml` | API backend (default) |

kubelet monitors these files and automatically starts/stops the corresponding
pods when the files change.

## Bootstrapped Components

The following components are deployed via the bootstrap kustomization after
kubeadm initialization:

| Component | Namespace | Purpose |
|-----------|-----------|---------|
| CoreDNS | kube-system | In-cluster DNS resolution |
| kube-flannel | kube-flannel | Pod networking (CNI) |
| kube-proxy | kube-system | Service proxy |
| metrics-server | kube-system | Resource metrics |
| local-path-provisioner | local-path-storage | Default StorageClass |
| kube-vip-cloud-provider | kube-system | LoadBalancer controller |

## Certificate Infrastructure

All Kubernetes certificates are stored in `/etc/kubernetes/pki/`:

| Certificate | Purpose |
|-------------|---------|
| `ca.crt` / `ca.key` | Kubernetes CA |
| `apiserver.crt` / `apiserver.key` | API server TLS |
| `etcd/ca.crt` | etcd CA |
| `front-proxy-ca.crt` | Front proxy CA |
| `sa.pub` / `sa.key` | Service account signing |
| `iknite-server.crt` / `iknite-server.key` | Iknite status server |
| `iknite-client.crt` / `iknite-client.key` | Iknite status client |

Certificates are valid for 1 year by default. Check expiry with:

```bash
kubeadm certs check-expiration
```

## Kubeconfig Files

| File | Purpose |
|------|---------|
| `/root/.kube/config` | Admin kubeconfig (kubectl) |
| `/etc/kubernetes/admin.conf` | Admin kubeconfig (original) |
| `/etc/kubernetes/kubelet.conf` | kubelet kubeconfig |
| `/etc/kubernetes/controller-manager.conf` | Controller manager kubeconfig |
| `/etc/kubernetes/scheduler.conf` | Scheduler kubeconfig |
| `/etc/kubernetes/iknite.conf` | Iknite status server client config |
| `/root/.kube/iknite.conf` | Iknite status server client config (home) |

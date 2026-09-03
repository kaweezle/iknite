!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Differences with kubeadm

Iknite wraps kubeadm but significantly modifies its behavior. This page
explains what Iknite changes compared to a standard kubeadm installation.

## Overview

| Feature | Standard kubeadm | Iknite |
|---------|-----------------|--------|
| Installation method | Shell binary | Go library (embedded) |
| kubelet management | systemd / OpenRC service | Subprocess of iknite |
| CoreDNS | Deployed by kubeadm addon phase | Deployed via kustomization |
| API backend | etcd only | kine (SQLite) by default, etcd optional |
| Node taint | Control plane taint applied | Taint removed (single-node) |
| VIP / Load balancer | Not included | Kube-VIP injected as static pod |
| WSL2 IP management | Not handled | Automatic secondary IP management |
| mDNS | Not included | Built-in pion/mdns responder |
| Post-init | Not included | Kustomization applied automatically |
| Workload wait | Not included | Polls until all workloads ready |
| Status tracking | Not included | IkniteCluster CR in /run/iknite/ |
| Status server | Not included | HTTPS status server on port 11443 |

## Initialization Phase Changes

Iknite modifies the kubeadm init workflow using Go's `//go:linkname` to access
unexported functions.

### Added Phases

| Phase | Description |
|-------|-------------|
| `iknite/prepare-containerd` | Cleans leftover containers and mounts |
| `iknite/kube-vip` | Injects Kube-VIP static pod manifest |
| `iknite/serve` | Starts the Iknite status HTTPS server |
| `iknite/workloads` | Applies bootstrap kustomization and waits |

### Removed / Suppressed Phases

| Phase | Reason |
|-------|--------|
| `addon/coredns` | CoreDNS is deployed via kustomization instead |

### Modified Phases

| Phase | Modification |
|-------|-------------|
| `kubelet-start` | kubelet is launched as a subprocess instead of via service |
| `mark-control-plane` | Node taint is removed to allow workload scheduling |

## Kubelet Management

Standard kubeadm relies on the system's init system (systemd on most distributions,
OpenRC on Alpine) to manage kubelet. Iknite takes a different approach:

### Standard kubeadm Flow

```
kubeadm init
  → writes kubelet config
  → systemctl start kubelet
  → kubelet starts and connects to API server
```

### Iknite Flow

```
iknite init
  → writes kubelet config
  → exec.Command("kubelet", flags...)
  → kubelet runs as a child process of iknite
  → on iknite stop: kubelet is sent SIGTERM
```

**Why?** This approach gives Iknite precise control over the kubelet lifecycle:
- Kubelet stops when iknite stops
- kubelet crashes can be detected and handled
- No race conditions between OpenRC service management and kubeadm init

To prevent kubelet from starting automatically via OpenRC, Iknite patches
`/etc/rc.conf`:

```bash
# Added by iknite to prevent kubelet auto-start
rc_keyword="!start" kubelet
```

## CoreDNS Deployment

Standard kubeadm deploys CoreDNS as part of the `addon` phase. Iknite suppresses
this and deploys CoreDNS via the bootstrap kustomization instead.

**Benefits:**
- CoreDNS configuration can be customized before deployment
- CoreDNS is managed alongside other add-ons consistently
- Versioning and patching is controlled by the kustomization

## API Backend: Kine instead of etcd

Standard kubeadm only supports etcd. Iknite's default is
[kine](https://github.com/k3s-io/kine), a lightweight etcd API implementation
backed by SQLite.

**Benefits of Kine:**
- ~100 MB lower memory usage compared to etcd
- No separate etcd process (runs as a static pod)
- Simpler backup (single SQLite file)
- Sufficient for development workloads

**Trade-offs:**
- Lower performance for high-write workloads
- Not suitable for multi-node clusters
- Fewer operational tools compared to etcd

## Kube-VIP Integration

Standard kubeadm does not include any load balancer. Iknite injects
[Kube-VIP](https://kube-vip.io/) as a static pod **before** the control plane
starts, so the virtual IP is available from the first API server connection.

Kube-VIP provides two functions:
1. **Control plane VIP**: A stable IP for the API server (useful in WSL2)
2. **Cloud provider**: `LoadBalancer` service type support

## Node Taint Removal

Standard kubeadm taints the control plane node with
`node-role.kubernetes.io/control-plane:NoSchedule`, preventing workloads from
being scheduled on it.

Iknite removes this taint since it is a **single-node cluster** that must run
both control plane and workload pods:

```bash
kubectl taint nodes --all node-role.kubernetes.io/control-plane-
```

## Reset Differences

Iknite's `iknite reset` (wrapping `kubeadm reset`) adds:
- Stopping the kubelet subprocess
- Cleaning up Iknite-specific configurations
- Optionally removing the virtual IP

`iknite clean` provides a more granular cleanup without requiring a full
kubeadm reset.

## State Tracking

Standard kubeadm has no post-initialization state tracking. Iknite persists
an `IkniteCluster` custom resource to `/run/iknite/status.json` tracking:
- Current initialization phase
- Workload readiness
- Cluster state transitions

This allows `iknite start` to detect and resume from partial initialization.

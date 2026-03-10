!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Features

Iknite provides a rich set of features on top of standard `kubeadm` to make
running a single-node Kubernetes cluster fast and easy.

## Core Components

### Kubernetes via kubeadm

Iknite wraps [kubeadm](https://kubernetes.io/docs/reference/setup-tools/kubeadm/)
to produce a **vanilla Kubernetes** cluster. This means you get:

- Full Kubernetes API compliance
- Standard manifests, certificates, and configuration
- Easy path to understanding production setups

### containerd Runtime

[containerd](https://containerd.io/) is the container runtime. It is installed
as an Alpine package dependency and managed via OpenRC. Iknite cleans up any
leftover pods and mounts from previous runs before starting.

### BuildKit

[BuildKit](https://docs.docker.com/build/buildkit/) is installed alongside
containerd so you can build container images directly inside the cluster without
Docker.

## Networking

### Flannel CNI

[Flannel](https://github.com/flannel-io/flannel) provides a simple overlay
network for pod-to-pod communication. It is deployed automatically as part of
the bootstrap kustomization.

### Kube-VIP

[Kube-VIP](https://kube-vip.io/) provides a virtual IP for the Kubernetes API
server (control plane) and a cloud-provider-style load balancer for `LoadBalancer`
services. This enables tools like Traefik or Argo CD to be accessed on a stable
IP without external load balancers.

The default virtual IP is `192.168.99.2` in WSL2 environments, bound to the
`eth0` interface.

## Storage

### Local Path Provisioner

[Local Path Provisioner](https://github.com/rancher/local-path-provisioner) by
Rancher provides a default `StorageClass` backed by the host filesystem
(`/opt/local-path-provisioner`). This lets you create `PersistentVolumeClaims`
without any additional storage setup.

The storage class is set as the **default** class automatically.

## Observability

### Metrics Server

[metrics-server](https://github.com/kubernetes-sigs/metrics-server) is deployed
as part of the bootstrap, enabling:

- `kubectl top nodes` and `kubectl top pods`
- [Horizontal Pod Autoscaling](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)
- Metrics in tools like [K9s](https://k9scli.io/)

## WSL2 and Incus / Stable IP

In WSL2 and Incus environments, the container/VM IP can change across restarts.
Iknite solves this by:

1. Adding a **stable secondary IP address** (`192.168.99.2` by default) to `eth0`
2. Registering the domain name `iknite.local` (or `kaweezle.local`) to this IP
3. Using this stable IP as the Kubernetes API server endpoint

This means `kubectl` configuration does not break across restarts.

### mDNS Responder

In WSL2 and Incus environments, Iknite launches a small mDNS responder so that
`iknite.local` resolves to the stable IP from the host. You can then use
`iknite.local` in your kubeconfig or browser.

## API Backend Options

By default Iknite uses [Kine](https://github.com/k3s-io/kine) as a lightweight
etcd replacement backed by SQLite. For users who need full etcd, Iknite also
supports the standard etcd backend.

### Kine (default)

- Backed by SQLite at `/var/lib/kine/kine.db`
- Much lower memory and CPU usage than etcd
- Compatible with the Kubernetes API

### etcd

Enable with `--use-etcd` flag or `useEtcd: true` in configuration:

- Standard etcd cluster with data at `/var/lib/etcd`
- Familiar tooling (`etcdctl`)

## Lifecycle Management

### OpenRC Integration

Iknite integrates with Alpine's [OpenRC](https://wiki.alpinelinux.org/wiki/OpenRC)
init system:

- `iknite` service in the `default` runlevel
- `containerd` and `buildkitd` services managed automatically
- `kubelet` is managed as a subprocess of iknite (not an OpenRC service) for
  better lifecycle control

### Cluster State Tracking

Iknite persists cluster state to `/run/iknite/status.json` in a Kubernetes-style
`IkniteCluster` custom resource format. This includes:

- Current initialization phase
- Workload readiness (ready/unready counts)
- Cluster state (`initializing`, `running`, etc.)

### Status Server

Iknite runs a small HTTPS status server on port `11443` with mTLS
authentication. The `iknite info status` command queries this server for live
cluster state.

## Bootstrap Kustomization

After kubeadm initializes the cluster, Iknite applies a
[kustomization](https://kustomize.io/) from `/etc/iknite.d/` (or a custom
directory). The default kustomization installs:

- CoreDNS
- Flannel CNI
- Local Path Provisioner
- Kube-VIP (control plane VIP + load balancer)
- Metrics Server

You can extend or replace this kustomization to pre-install any workloads you
need.

## Pre-pulled Container Images

The optional `iknite-images` APK package pre-imports all container images
needed by Kubernetes and the bootstrap components, dramatically reducing
first-boot time (by 2–5 minutes).

## Summary Table

| Feature | Implementation | Default |
|---------|---------------|---------|
| Container runtime | containerd | ✅ |
| Image building | BuildKit | ✅ |
| CNI | Flannel | ✅ |
| Load balancer / VIP | Kube-VIP | ✅ |
| Storage class | Local Path Provisioner | ✅ |
| Metrics | metrics-server | ✅ |
| API backend | Kine (SQLite) | ✅ |
| API backend (alternative) | etcd | Optional |
| Init system | OpenRC | ✅ |
| Stable IP (WSL2 / Incus) | Secondary NIC address | WSL2 and Incus |
| mDNS | pion/mdns | WSL2 and Incus |

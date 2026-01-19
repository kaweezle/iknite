!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

![iknite logo](img/logo.svg){ width=200, align=right }

# Iknite

**Iknite** is a flexible Golang CLI that simplifies creating a Kubernetes
development cluster in an Alpine Linux–based VM, LXC container, or WSL2
environment.

As opposed to other tools, Iknite encapsulates `kubeadm` in order to simplify
the process of creating a development cluster.

As mentioned in the
[kubeadm documentation](https://kubernetes.io/docs/reference/setup-tools/kubeadm/):

!!! quote "kubeadm documentation"

    Instead, we expect higher-level and more tailored tooling to be built on top
    of kubeadm, and ideally, using kubeadm as the basis of all deployments will
    make it easier to create conformant clusters.

**Iknite** comes as an Alpine Linux package, installing among others `kubeadm`,
`kubectl`, `kubelet` automatically as dependencies.

Using [containerd](https://containerd.io/) as the runtime, it also deploys
[buildkit](https://docs.docker.com/build/buildkit/) for straightforward image
building.

On launch, **Iknite** creates a single control plane node that doubles as a
worker node, then applies a **bootstrapping
[kustomization](https://kustomize.io/)** to the cluster to install the
appropriate tools and configs.

A WSL2 distribution is available for quickly spinning up clusters on Windows
without relying on Docker.

**Iknite** is best suited for development. It is not intended for production
use.

## Similar tools

- [kind](https://kind.sigs.k8s.io/): Kubernetes IN Docker - a tool for running
  local Kubernetes clusters using Docker container "nodes".
- [minikube](https://minikube.sigs.k8s.io/): Run Kubernetes locally.
- [k3d](https://k3d.io/): Run k3s in Docker.
- [k3s](https://k3s.io/): Lightweight Kubernetes.

`kind`, `k3d` and `k3s` assume that you already have Docker installed.
`minikube` doesn't provide a WSL driver and in consequence also requires Docker
on Windows or a Hyper-V VM coming in addition to the WSL2 backing VM.

Unlike these tools, **Iknite** targets WSL2 on Windows without Docker. Its
bootstrapping kustomization simplifies experimenting with various Kubernetes
deployment setups.

**Iknite** can also be run on a lightweight cloud based Alpine Linux VM. A
[cloud-init](https://cloud-init.io/) compatible image will be available soon.

#!/bin/zsh

set -ex

KUBERNETES_VERSION=$1

cd /root
apk update --quiet
apk add --no-progress --no-cache fuse-overlayfs
wget https://github.com/containerd/fuse-overlayfs-snapshotter/releases/download/v1.0.8/containerd-fuse-overlayfs-1.0.8-linux-amd64.tar.gz
tar -xvf containerd-fuse-overlayfs-1.0.8-linux-amd64.tar.gz

./containerd-fuse-overlayfs-grpc /var/run/fuse-overlayfs.sock /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs >/var/log/fuse-overlayfs.log 2>&1 &

cp /etc/containerd/config.toml /etc/containerd/config.toml.bak

sed -i -e '/\[proxy_plugins\]/d' /etc/containerd/config.toml

cat - >> /etc/containerd/config.toml <<EOF

[proxy_plugins]
  [proxy_plugins."fuse-overlayfs"]
    type = "snapshot"
    address = "/var/run/fuse-overlayfs.sock"
EOF

export CONTAINERD_SNAPSHOTTER=fuse-overlayfs
containerd >/var/log/containerd.log 2>&1 &
sleep 3

nerdctl --namespace=k8s.io pull -q registry.k8s.io/pause:3.10
nerdctl --namespace=k8s.io pull -q registry.k8s.io/pause:3.9
nerdctl --namespace=k8s.io pull -q registry.k8s.io/etcd:3.5.12-0
nerdctl --namespace=k8s.io pull -q registry.k8s.io/kube-controller-manager:v${KUBERNETES_VERSION}
nerdctl --namespace=k8s.io pull -q registry.k8s.io/kube-scheduler:v${KUBERNETES_VERSION}
nerdctl --namespace=k8s.io pull -q registry.k8s.io/kube-apiserver:v${KUBERNETES_VERSION}
nerdctl --namespace=k8s.io pull -q registry.k8s.io/kube-proxy:v${KUBERNETES_VERSION}
nerdctl --namespace=k8s.io pull -q registry.k8s.io/coredns/coredns:v1.11.1
nerdctl --namespace=k8s.io pull -q docker.io/rancher/local-path-provisioner:v0.0.31
nerdctl --namespace=k8s.io pull -q registry.k8s.io/metrics-server/metrics-server:v0.7.2
nerdctl --namespace=k8s.io pull -q ghcr.io/flannel-io/flannel:v0.26.5
nerdctl --namespace=k8s.io pull -q ghcr.io/boxboat/kubectl:${KUBERNETES_VERSION}
nerdctl --namespace=k8s.io pull -q ghcr.io/kube-vip/kube-vip:v0.8.9
nerdctl --namespace=k8s.io pull -q ghcr.io/kube-vip/kube-vip-cloud-provider:v0.0.11

kill %1 %2
rm -f /var/log/containerd.log /var/log/fuse-overlayfs.log
mv /etc/containerd/config.toml.bak /etc/containerd/config.toml
rm -f containerd-fuse-overlayfs-grpc containerd-fuse-overlayfs-1.0.8-linux-amd64.tar.gz
apk del fuse-overlayfs
rm -rf `find /var/cache/apk/ -type f`

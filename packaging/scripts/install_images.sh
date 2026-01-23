#!/bin/sh
# cSpell: words lowerdir upperdir fusermount overlayfs
# Script to pre-download container images into the image being built.
# This allows air-gapped installations to have all required images
# already present.
# Docker and containerd use overlayfs for storage. It is not possible
# to pull images using overlayfs inside a container that itself uses
# overlayfs (because of kernel restrictions). To work around this,
# we use the fuse-overlayfs snapshotter for containerd to pull the images.
# The pulled images are stored in the overlayfs storage, and when the
# snapshotter process is killed, the images remain available for use with
# the standard overlayfs snapshotter. This is useful when exporting images
# into a VM image or a WSL2 image.
# Usage: install_images.sh <image-list-file>
set -ex

# List of images to download
IMAGE_LIST_FILE=$1

# Install and start containerd-fuse-overlayfs-snapshotter to pull images using fuse-overlayfs
cd /root
apk update --quiet
apk add --no-progress --no-cache fuse-overlayfs
wget https://github.com/containerd/fuse-overlayfs-snapshotter/releases/download/v1.0.8/containerd-fuse-overlayfs-1.0.8-linux-amd64.tar.gz
tar -xvf containerd-fuse-overlayfs-1.0.8-linux-amd64.tar.gz

./containerd-fuse-overlayfs-grpc /var/run/fuse-overlayfs.sock /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs >/var/log/fuse-overlayfs.log 2>&1 &

# Configure containerd to use the fuse-overlayfs snapshotter
cp /etc/containerd/config.toml /etc/containerd/config.toml.bak

sed -i -e '/\[proxy_plugins\]/d' /etc/containerd/config.toml

cat - >> /etc/containerd/config.toml <<EOF

[proxy_plugins]
  [proxy_plugins."fuse-overlayfs"]
    type = "snapshot"
    address = "/var/run/fuse-overlayfs.sock"
EOF

# Start containerd with the new configuration
containerd >/var/log/containerd.log 2>&1 &
sleep 3

# Pull images using nerdctl and the fuse-overlayfs snapshotter
export CONTAINERD_SNAPSHOTTER=fuse-overlayfs
cat "${IMAGE_LIST_FILE}" | while read -r IMAGE; do
    nerdctl --namespace=k8s.io pull -q "${IMAGE}"
done

# Kill the containerd and fuse-overlayfs snapshotter processes
kill %1 %2
# Cleanup
rm -f /var/log/containerd.log /var/log/fuse-overlayfs.log
mv /etc/containerd/config.toml.bak /etc/containerd/config.toml
rm -f containerd-fuse-overlayfs-grpc containerd-fuse-overlayfs-1.0.8-linux-amd64.tar.gz
apk del fuse-overlayfs
rm -rf "$(find /var/cache/apk/ -type f)"

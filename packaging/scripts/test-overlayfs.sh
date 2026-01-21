#!/bin/sh
# shellcheck disable=SC2329,SC3037
# cSpell: words lowerdir upperdir fusermount overlayfs mountpoint
set -e

# This script checks if fuse-overlayfs is needed by testing if the overlay filesystem can be mounted directly.
is_fuse_overlayfs_needed() {
    local tmpdir
    tmpdir=$(mktemp -d)
    mkdir -p "$tmpdir/lower" "$tmpdir/upper" "$tmpdir/work" "$tmpdir/merged"
    echo -n "test" > "$tmpdir/lower/hello.txt"

    if mount -t overlay overlay -o lowerdir="${tmpdir}/lower,upperdir=${tmpdir}/upper,workdir=${tmpdir}/work" "${tmpdir}/merged" 2>/dev/null; then
        if mountpoint -q "${tmpdir}/merged"; then
            umount "${tmpdir}/merged"
            rm -rf "$tmpdir"
            return 1  # echo "no"
        fi
    fi
    rm -rf "$tmpdir"
    return 0  # echo "yes"
}

# This function installs fuse-overlayfs and checks if it works by creating a simple overlay filesystem.
add_and_check_fuse_overlayfs() {
    apk add --update --no-progress --no-cache fuse-overlayfs
    local tmpdir
    tmpdir=$(mktemp -d)
    local result=1
    mkdir -p "$tmpdir/lower" "$tmpdir/upper" "$tmpdir/work" "$tmpdir/merged"
    echo -n "test" > "$tmpdir/lower/hello.txt"

    if fuse-overlayfs -o lowerdir="${tmpdir}/lower,upperdir=${tmpdir}/upper,workdir=${tmpdir}/work" "${tmpdir}/merged" 2>/dev/null; then
        if [ -f "${tmpdir}/merged/hello.txt" ] && [ "$(cat "${tmpdir}/merged/hello.txt")" = "test" ]; then
            result=0
        fi
        fusermount3 -u "${tmpdir}/merged"
    fi
    rm -rf "$tmpdir"
    return "$result"
}

# This function removes fuse-overlayfs after testing.
remove_fuse_overlayfs() {
    apk del fuse-overlayfs
}

install_fuse_overlayfs_snapshotter() {

    echo "Installing fuse-overlayfs snapshotter"
    (cd /usr/local/bin && \
        wget -q -O - https://github.com/containerd/fuse-overlayfs-snapshotter/releases/download/v1.0.8/containerd-fuse-overlayfs-1.0.8-linux-amd64.tar.gz | tar zx)

    echo "Starting fuse-overlayfs snapshotter"
    /usr/local/bin/containerd-fuse-overlayfs-grpc /var/run/fuse-overlayfs.sock /var/lib/containerd/io.containerd.snapshotter.v1.overlayfs >/var/log/fuse-overlayfs.log 2>&1 &

    echo "Configuring containerd to use fuse-overlayfs"
    # Backup the original config file
    if [ -f /etc/containerd/config.toml.bak ]; then
        echo "Backup already exists, skipping backup"
    else
        cp /etc/containerd/config.toml /etc/containerd/config.toml.bak
    fi
    # Remove any existing proxy_plugins section
    if grep -q "\[proxy_plugins\]" /etc/containerd/config.toml; then
        sed -i -e '/\[proxy_plugins\]/d' /etc/containerd/config.toml
    fi
    # Add the new proxy_plugins section
    cat - >> /etc/containerd/config.toml <<EOF

[proxy_plugins]
  [proxy_plugins."fuse-overlayfs"]
    type = "snapshot"
    address = "/var/run/fuse-overlayfs.sock"
EOF
    echo "Containerd snapshotter set to $CONTAINERD_SNAPSHOTTER"
}

remove_fuse_overlayfs_snapshotter() {
    echo "Removing fuse-overlayfs snapshotter"
    kill %
    rm -f /var/log/fuse-overlayfs.log
    mv /etc/containerd/config.toml.bak /etc/containerd/config.toml
    rm -f /usr/local/bin/containerd-fuse-overlayfs-grpc
    unset CONTAINERD_SNAPSHOTTER
    echo "Containerd snapshotter removed"
}

start_containerd() {
    echo "Starting containerd..."
    containerd --config /etc/containerd/config.toml >/dev/null 2>&1 &
    # Wait for containerd to start
    while [ ! -S /run/containerd/containerd.sock ]; do
        sleep 1
    done
}

stop_containerd() {
    echo "Stopping containerd..."
    kill %
    rm -f /run/containerd/containerd.sock
}

RESULT=0

used_fuse=no
# apk add --no-progress --no-cache containerd

# if is_fuse_overlayfs_needed; then
#     echo "fuse-overlayfs is needed"
#     if add_and_check_fuse_overlayfs; then
#         echo "fuse-overlayfs is working"
#         used_fuse=yes
#         install_fuse_overlayfs_snapshotter
#     else
#         echo "fuse-overlayfs is not working"
#         RESULT=1
#     fi
# else
#     echo "fuse-overlayfs is not needed"
# fi

# if [ "$used_fuse" = "yes" ]; then
#     CONTAINERD_SNAPSHOTTER=fuse-overlayfs start_containerd
# else
#     start_containerd
# fi

apk add --allow-untrusted --no-cache /root/iknite-images-1.35.0.x86_64.apk

# cd /usr/share/iknite/images
# for i in *.tar.gz; do
#     name=$(basename $i .tar.gz | tr '_' ':')
#     # Check if the image is already imported
#     if ! ctr -n k8s.io images ls | grep -q "$name"; then
#         # Import the image
#         echo "Importing container image $name..."
#         ctr -n k8s.io images import $i
#     else
#         echo "Container image $name already imported, skipping."
#     fi
# done

# stop_containerd
apk del iknite-images

if [ "$used_fuse" = "yes" ]; then
    remove_fuse_overlayfs_snapshotter
    remove_fuse_overlayfs
fi

exit $RESULT

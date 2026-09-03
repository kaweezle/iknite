<!-- cSpell: words netdev hostfwd nographic netfilter nftables veth -->

!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Other / Rootless Environments

This page covers deploying Iknite in environments not specifically listed
elsewhere, including other Linux distributions, rootless containers, and minimal
VM setups.

## Alpine Linux (Direct Installation)

If you are already running Alpine Linux (in a VM, bare metal, or container),
install Iknite directly from the APK repository.

### Add the Repository

```bash
# Add the Iknite APK repository
echo "https://kaweezle.com/repo/kaweezle/x86_64" >> /etc/apk/repositories

# Add the signing key
wget -O /etc/apk/keys/kaweezle-devel@kaweezle.com-c9d89864.rsa.pub \
  https://kaweezle.com/repo/kaweezle/kaweezle-devel@kaweezle.com-c9d89864.rsa.pub

apk update
```

### Install Iknite

```bash
# Install iknite and dependencies
apk add iknite

# Optional: pre-pull container images for faster first boot
apk add iknite-images
```

### Configure and Start

```bash
# Configure for your environment
cat > /etc/iknite.d/iknite.yaml <<EOF
createIp: false
enableMDNS: false
clusterName: "my-cluster"
useEtcd: false
EOF

# Enable the OpenRC service
rc-update add iknite default
rc-update add containerd default

# Start all services
openrc default

# Or start directly
/sbin/iknite start -t 120
```

## Rootless Environments

!!! warning "Limited support" Kubernetes requires several Linux capabilities and
kernel features that may not be available in all rootless environments. Rootless
support is experimental.

### Requirements for Rootless Operation

- Linux kernel 5.11+
- User namespaces enabled (`/proc/sys/user/max_user_namespaces > 0`)
- cgroups v2 enabled
- `newuidmap` / `newgidmap` available

### Rootless containerd

```bash
# Set up rootless containerd
containerd-rootless-setuptool.sh install

# Configure Iknite to use rootless containerd socket
cat > /etc/crictl.yaml <<EOF
runtime-endpoint: unix:///run/user/1000/containerd/containerd.sock
image-endpoint: unix:///run/user/1000/containerd/containerd.sock
timeout: 30
debug: false
EOF
```

## LXC Containers (Without Incus)

For raw LXC containers without Incus:

### Create a Privileged Container

```bash
# Create a container
lxc-create -n iknite -t download -- -d alpine -r edge -a amd64

# Configure for Kubernetes
cat >> /var/lib/lxc/iknite/config <<EOF
lxc.apparmor.profile = unconfined
lxc.cap.drop =
lxc.cgroup.devices.allow = a
lxc.mount.auto = proc:rw sys:rw cgroup:rw
EOF

# Start the container
lxc-start -n iknite

# Install Iknite
lxc-attach -n iknite -- apk add iknite
lxc-attach -n iknite -- /sbin/iknite start -t 120
```

## QEMU/KVM

Use the QCOW2 image directly with QEMU:

```bash
# Download the QCOW2 image
curl -LO "https://github.com/kaweezle/iknite/releases/latest/download/iknite.qcow2"

# Create a backing image to preserve the base
qemu-img create -f qcow2 -F qcow2 -b iknite.qcow2 my-cluster.qcow2

# Launch the VM
qemu-system-x86_64 \
  -m 4G \
  -smp 2 \
  -hda my-cluster.qcow2 \
  -netdev user,id=net0,hostfwd=tcp::6443-:6443,hostfwd=tcp::2222-:22 \
  -device virtio-net,netdev=net0 \
  -nographic
```

## General Configuration for Non-WSL2 Environments

When deploying in environments where the host IP is stable (VMs, bare metal):

```yaml
# /etc/iknite.d/iknite.yaml
createIp: false # Don't create a secondary IP
enableMDNS: false # No mDNS needed
domainName: "" # Use the actual host IP
clusterName: "my-cluster"
useEtcd: false
```

## Environment Variables for Containerized Deployments

When running inside a container, you can configure Iknite entirely via
environment variables:

```bash
docker run \
  --privileged \
  -e IKNITE_CREATE_IP=false \
  -e IKNITE_ENABLE_MDNS=false \
  -e IKNITE_CLUSTER_NAME=my-cluster \
  -e IKNITE_USE_ETCD=false \
  iknite:latest
```

## Kernel Requirements

Regardless of the deployment target, the host kernel must support:

| Feature                  | Required For                   |
| ------------------------ | ------------------------------ |
| `overlay` filesystem     | containerd layer storage       |
| `br_netfilter`           | Kubernetes networking          |
| `ip_tables` / `nftables` | Pod networking (Flannel)       |
| User namespaces          | containerd rootless mode       |
| cgroups v2               | Kubelet resource management    |
| `veth` devices           | Pod virtual network interfaces |

```bash
# Check required kernel modules
lsmod | grep -E "overlay|br_netfilter"

# Load if missing
modprobe overlay
modprobe br_netfilter

# Persist across reboots
echo "overlay" >> /etc/modules
echo "br_netfilter" >> /etc/modules
```

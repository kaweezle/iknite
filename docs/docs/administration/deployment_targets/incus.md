!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Incus Deployment

[Incus](https://linuxcontainers.org/incus/) is an open-source container and
VM manager, the community fork of LXD. Iknite runs well in Incus containers
with `security.nesting` enabled.

## Prerequisites

- Linux host with Incus 6.0+
- At least 4 GB RAM available for the container
- `incus` CLI installed and initialized

## Installation

### 1. Install and Initialize Incus

```bash
# On Debian/Ubuntu
curl https://pkgs.zabbly.com/get/incus-stable | sudo sh -s

# Initialize Incus (interactive)
sudo incus admin init

# Or minimal init
sudo incus admin init --minimal
```

### 2. Import the Iknite Image

```bash
# Download the rootfs
curl -LO "https://github.com/kaweezle/iknite/releases/latest/download/iknite-rootfs.tar.gz"

# Import as an Incus image
incus image import iknite-rootfs.tar.gz --alias iknite
```

### 3. Create a Container Profile

Create an Incus profile for Iknite containers with the required security settings:

```bash
incus profile create iknite
incus profile edit iknite <<EOF
config:
  security.nesting: "true"
  security.privileged: "true"
  limits.memory: 8GB
  limits.cpu: "4"
description: Iknite Kubernetes container profile
devices:
  eth0:
    name: eth0
    nictype: bridged
    parent: incusbr0
    type: nic
  root:
    path: /
    pool: default
    size: 20GB
    type: disk
EOF
```

### 4. Launch the Container

```bash
incus launch iknite my-cluster --profile iknite
```

### 5. Initialize the Cluster

```bash
# Wait for the container to start
incus exec my-cluster -- sleep 5

# Start the cluster
incus exec my-cluster -- /sbin/iknite start -t 120
```

### 6. Access the Cluster

```bash
# Copy the kubeconfig
incus file pull my-cluster/root/.kube/config /tmp/iknite-config

# Access the cluster
KUBECONFIG=/tmp/iknite-config kubectl get nodes
```

## Network Configuration

### Static IP Address

To give the container a static IP:

```bash
incus config device override my-cluster eth0 ipv4.address=192.168.1.10
```

Update Iknite configuration to not create a secondary IP:

```bash
incus exec my-cluster -- sh -c 'cat > /etc/iknite.d/iknite.yaml <<EOF
domainName: ""
ip: "192.168.1.10"
createIp: false
enableMDNS: false
EOF'
```

### Port Forwarding

If the container does not have a routable IP, use port forwarding:

```bash
incus config device add my-cluster apiserver proxy \
  listen=tcp:0.0.0.0:6443 connect=tcp:127.0.0.1:6443
```

## Auto-Start

```bash
# Enable auto-start on host boot
incus config set my-cluster boot.autostart true
incus config set my-cluster boot.autostart.delay 0
```

Enable OpenRC inside the container for auto-start on container boot:

```bash
incus exec my-cluster -- rc-update add iknite default
```

## Snapshots and Backup

```bash
# Take a snapshot
incus snapshot create my-cluster clean-state

# Restore a snapshot
incus snapshot restore my-cluster clean-state

# Export the container
incus export my-cluster /backup/my-cluster.tar.gz
```

## Troubleshooting

### Kernel Modules

Some Kubernetes features require kernel modules on the Incus host:

```bash
modprobe br_netfilter
modprobe overlay
```

### Security.nesting

If Kubernetes pods fail to start, ensure `security.nesting` is enabled:

```bash
incus config set my-cluster security.nesting true
incus restart my-cluster
```

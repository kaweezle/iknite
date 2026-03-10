!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# OpenStack Deployment

Iknite provides QCOW2 VM images compatible with OpenStack. This page explains
how to deploy and manage Iknite on OpenStack.

## Prerequisites

- Access to an OpenStack cluster (Horizon or CLI)
- `openstack` CLI installed and configured
- At least m1.large flavor (4 vCPU, 8 GB RAM) recommended

## Downloading the Image

Download the latest QCOW2 image from the
[releases page](https://github.com/kaweezle/iknite/releases):

```bash
curl -LO "https://github.com/kaweezle/iknite/releases/latest/download/iknite.qcow2"
```

## Uploading the Image to OpenStack

```bash
# Upload the image to OpenStack Glance
openstack image create "iknite" \
  --file iknite.qcow2 \
  --disk-format qcow2 \
  --container-format bare \
  --property hw_firmware_type=uefi \
  --public
```

## Launching a VM

### Via CLI

```bash
# Create a security group for Kubernetes
openstack security group create kubernetes
openstack security group rule create kubernetes --protocol tcp --dst-port 6443
openstack security group rule create kubernetes --protocol tcp --dst-port 22
openstack security group rule create kubernetes --protocol tcp --dst-port 11443
openstack security group rule create kubernetes --protocol icmp

# Launch the VM
openstack server create \
  --image iknite \
  --flavor m1.large \
  --network my-network \
  --security-group kubernetes \
  --security-group default \
  --key-name my-key \
  iknite-cluster
```

### With Cloud-Init

Cloud-init is fully supported. Customize the startup configuration:

```yaml
# cloud-init.yaml
#cloud-config
write_files:
  - path: /etc/iknite.d/iknite.yaml
    content: |
      domainName: ""
      createIp: false
      enableMDNS: false
      clusterName: "my-openstack-cluster"
      useEtcd: false

runcmd:
  - [rc-update, add, iknite, default]
  - [openrc, default]
```

```bash
openstack server create \
  --image iknite \
  --flavor m1.large \
  --network my-network \
  --security-group kubernetes \
  --key-name my-key \
  --user-data cloud-init.yaml \
  iknite-cluster
```

## Assigning a Floating IP

```bash
# Associate a floating IP for external access
openstack floating ip create public
openstack server add floating ip iknite-cluster <floating-ip>
```

## Configuring Iknite for OpenStack

In an OpenStack VM, the IP is assigned by DHCP and is stable (not dynamic like
WSL2). Disable IP creation and mDNS:

```yaml
# /etc/iknite.d/iknite.yaml
createIp: false         # Use the DHCP-assigned IP
enableMDNS: false       # No mDNS needed
domainName: ""          # Use actual IP or configure DNS separately
clusterName: "my-cluster"
useEtcd: false
```

## Accessing the Cluster

```bash
# SSH to the VM
ssh root@<floating-ip>

# Copy kubeconfig to local machine
scp root@<floating-ip>:/root/.kube/config ~/.kube/openstack-iknite

# Access the cluster
KUBECONFIG=~/.kube/openstack-iknite kubectl get nodes
```

## Infrastructure as Code

The Iknite project uses Terraform/Terragrunt for OpenStack VM management. See
the `deploy/iac/iknite/iknite-vm/` directory for the Terraform configuration.

```hcl
# Example Terraform usage
module "iknite_vm" {
  source = "./modules/openstack-vm"

  instances = {
    "my-cluster" = {
      name         = "iknite-cluster"
      image_name   = "iknite"
      flavor_name  = "m1.large"
      network_name = "my-network"
    }
  }
}
```

## Volumes and Persistent Storage

Attach persistent storage for cluster data:

```bash
# Create a volume for etcd/kine data
openstack volume create --size 20 iknite-data

# Attach to the VM
openstack server add volume iknite-cluster iknite-data

# Inside the VM: mount and configure
mkfs.ext4 /dev/vdb
mount /dev/vdb /var/lib/kine
echo "/dev/vdb /var/lib/kine ext4 defaults 0 0" >> /etc/fstab
```

## Troubleshooting

### Cloud-Init Not Running

Check cloud-init logs:

```bash
cat /var/log/cloud-init-output.log
cloud-init status
```

### VM Cannot Connect to Kubernetes API

Ensure the security group allows port 6443:

```bash
openstack security group rule list kubernetes
```

### Image Import Fails

Ensure the image format is `qcow2` and the image is not corrupted:

```bash
qemu-img info iknite.qcow2
qemu-img check iknite.qcow2
```

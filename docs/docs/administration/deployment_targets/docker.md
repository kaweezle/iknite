!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Docker Deployment

!!! warning "Docker support is ongoing"
    Docker deployment support is currently under active development. Some
    features may not work as expected. Follow the progress at
    [GitHub issue #182](https://github.com/kaweezle/iknite/issues/182).

Iknite can run inside a Docker container in privileged mode. This deployment
target is useful on Linux, macOS, and Windows when Docker is available.

## Prerequisites

- Docker 20.10+ installed
- At least 4 GB RAM allocated to Docker

!!! warning "Privileged mode required"
    Running Kubernetes inside Docker requires the `--privileged` flag. This
    grants the container full access to the host kernel, which is a security
    consideration for production environments.

## Running with Docker

### Basic Start

```bash
docker run \
  --privileged \
  --name iknite \
  -d \
  ghcr.io/kaweezle/iknite:latest /sbin/iknite init
```

### With Persistent Storage

To preserve cluster data across container restarts:

```bash
docker run \
  --privileged \
  --name iknite \
  -d \
  -v iknite-etcd:/var/lib/kine \
  -v iknite-kubelet:/var/lib/kubelet \
  -v iknite-k8s:/etc/kubernetes \
  -v iknite-pv:/opt/local-path-provisioner \
  ghcr.io/kaweezle/iknite:latest /sbin/iknite init
```

### With Port Mapping

```bash
docker run \
  --privileged \
  --name iknite \
  -d \
  -p 6443:6443 \
  -p 11443:11443 \
  -p 80:80 \
  -p 443:443 \
  ghcr.io/kaweezle/iknite:latest /sbin/iknite init
```

Port `11443` is the Iknite status server port (mTLS, used by `iknite info status`).

## Accessing the Cluster

```bash
# Copy kubeconfig
docker cp iknite:/root/.kube/config ~/.kube/iknite-docker-config

# Access the cluster
export KUBECONFIG=~/.kube/iknite-docker-config
kubectl get nodes
```

## Docker Compose

```yaml
# docker-compose.yaml
version: "3.8"

services:
  iknite:
    image: ghcr.io/kaweezle/iknite:latest
    container_name: iknite
    privileged: true
    command: ["/sbin/iknite", "init"]
    ports:
      - "6443:6443"
      - "11443:11443"
      - "80:80"
      - "443:443"
    volumes:
      - iknite-kine:/var/lib/kine
      - iknite-kubelet:/var/lib/kubelet
      - iknite-k8s:/etc/kubernetes
      - iknite-pv:/opt/local-path-provisioner
    restart: unless-stopped

volumes:
  iknite-kine:
  iknite-kubelet:
  iknite-k8s:
  iknite-pv:
```

```bash
docker compose up -d
```

## Configuration

### Custom Iknite Configuration

Mount a custom configuration file:

```bash
docker run \
  --privileged \
  --name iknite \
  -d \
  -v ./iknite.yaml:/etc/iknite.d/iknite.yaml:ro \
  ghcr.io/kaweezle/iknite:latest
```

Example `iknite.yaml` for Docker:

```yaml
createIp: false     # Docker manages networking
enableMDNS: false   # No mDNS in Docker
domainName: ""      # Use container IP
clusterName: "docker-cluster"
useEtcd: false
```

### Custom Kustomization

```bash
docker run \
  --privileged \
  --name iknite \
  -d \
  -v ./my-kustomization:/etc/iknite.d:ro \
  ghcr.io/kaweezle/iknite:latest
```

## Stopping and Cleaning Up

```bash
# Stop the container (triggers graceful cluster shutdown)
docker stop iknite

# Remove the container
docker rm iknite

# Remove all data volumes (full reset)
docker volume rm iknite-kine iknite-kubelet iknite-pv
```

## Building the Image from Source

```bash
# Clone the repository
git clone https://github.com/kaweezle/iknite.git
cd iknite

# Build the rootfs image
./packaging/scripts/build-helper.sh --only-build --with-cache

# The image is available as ghcr.io/kaweezle/iknite:latest
```

## Limitations

- **No GPU support**: GPU passthrough requires additional configuration
- **No LoadBalancer from outside**: Kube-VIP LoadBalancer IPs are only
  accessible within the Docker network unless port-mapped
- **Privileged only**: Docker security constraints require `--privileged`
- **macOS performance**: Filesystem performance on macOS may be slower due to
  the Docker Desktop VM layer

## Troubleshooting

### Container Exits Immediately

Check the container logs:

```bash
docker logs iknite
```

Ensure `--privileged` is set.

### kubectl Connection Refused

Ensure port 6443 is mapped and update the kubeconfig server address:

```bash
# Update server address in kubeconfig
kubectl config set-cluster iknite --server=https://localhost:6443
```

### Pods Not Starting

In some Docker environments, kernel capabilities may be limited. Try:

```bash
docker run \
  --privileged \
  --cap-add=NET_ADMIN \
  --cap-add=SYS_ADMIN \
  --cap-add=SYS_PTRACE \
  --name iknite \
  -d \
  ghcr.io/kaweezle/iknite:latest
```

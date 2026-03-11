!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Configuration

This page explains the Iknite configuration file and all available options.

## Configuration File Locations

Iknite uses [Viper](https://github.com/spf13/viper) for configuration, with the
following search paths (first found wins):

1. Path specified with `--config` flag
2. `~/.config/iknite/iknite.yaml`
3. `/etc/iknite.d/iknite.yaml`

## Configuration Priority

Options are applied in this order (highest priority first):

1. **CLI flags** (e.g., `--domain-name iknite.local`)
2. **Environment variables** (prefix `IKNITE_`, e.g., `IKNITE_DOMAIN_NAME`)
3. **Configuration file** (`iknite.yaml`)
4. **Built-in defaults**

## Configuration Options Reference

### `kubernetesVersion`

The Kubernetes version to use for cluster initialization.

| Property | Value                                            |
| -------- | ------------------------------------------------ |
| Type     | `string`                                         |
| Default  | Compiled-in version (see `iknite info versions`) |
| CLI flag | `--kubernetes-version`                           |
| Env var  | `IKNITE_KUBERNETES_VERSION`                      |

```yaml
kubernetesVersion: "1.35.0"
```

### `domainName`

The hostname used as the Kubernetes API server address. Iknite registers this
name in `/etc/hosts` and optionally via mDNS.

| Property | Value                |
| -------- | -------------------- |
| Type     | `string`             |
| Default  | `iknite.local`       |
| CLI flag | `--domain-name`      |
| Env var  | `IKNITE_DOMAIN_NAME` |

```yaml
domainName: "iknite.local"
```

### `ip`

The IP address to use as the Kubernetes API server endpoint. In WSL2
environments, this is typically a secondary IP added to the `eth0` interface.

| Property | Value                 |
| -------- | --------------------- |
| Type     | `string` (IP address) |
| Default  | `192.168.99.2`        |
| CLI flag | `--ip`                |
| Env var  | `IKNITE_IP`           |

```yaml
ip: "192.168.99.2"
```

### `createIp`

Whether to add the configured `ip` as a secondary address to the network
interface. Enable this in WSL2 to ensure the IP is always available.

| Property | Value              |
| -------- | ------------------ |
| Type     | `bool`             |
| Default  | `true`             |
| CLI flag | `--create-ip`      |
| Env var  | `IKNITE_CREATE_IP` |

```yaml
createIp: true
```

### `networkInterface`

The network interface to add the secondary IP address to.

| Property | Value                      |
| -------- | -------------------------- |
| Type     | `string`                   |
| Default  | `eth0`                     |
| CLI flag | `--network-interface`      |
| Env var  | `IKNITE_NETWORK_INTERFACE` |

```yaml
networkInterface: "eth0"
```

### `clusterName`

The name of the Kubernetes cluster. Used in kubeconfig contexts and
certificates.

| Property | Value                 |
| -------- | --------------------- |
| Type     | `string`              |
| Default  | `iknite`              |
| CLI flag | `--cluster-name`      |
| Env var  | `IKNITE_CLUSTER_NAME` |

```yaml
clusterName: "iknite"
```

### `kustomization`

Path to the directory containing the bootstrap kustomization files.

| Property | Value                  |
| -------- | ---------------------- |
| Type     | `string`               |
| Default  | `/etc/iknite.d`        |
| CLI flag | `--kustomization`      |
| Env var  | `IKNITE_KUSTOMIZATION` |

```yaml
kustomization: "/etc/iknite.d"
```

### `useEtcd`

Use standard etcd instead of Kine as the API backend.

| Property | Value                         |
| -------- | ----------------------------- |
| Type     | `bool`                        |
| Default  | `false` (Kine is the default) |
| CLI flag | `--use-etcd`                  |
| Env var  | `IKNITE_USE_ETCD`             |

```yaml
useEtcd: false
```

### `apiBackendDatabaseDirectory`

Custom directory for the API backend database files.

| Property | Value                                            |
| -------- | ------------------------------------------------ |
| Type     | `string`                                         |
| Default  | `/var/lib/kine` (Kine) or `/var/lib/etcd` (etcd) |
| CLI flag | `--api-backend-database-directory`               |
| Env var  | `IKNITE_API_BACKEND_DATABASE_DIRECTORY`          |

```yaml
apiBackendDatabaseDirectory: "/var/lib/kine"
```

### `enableMDNS`

Whether to enable the mDNS responder for the cluster domain name. Useful in WSL2
for resolving the domain name from Windows.

| Property | Value                       |
| -------- | --------------------------- |
| Type     | `bool`                      |
| Default  | `true` in WSL2 environments |
| CLI flag | `--enable-mdns`             |
| Env var  | `IKNITE_ENABLE_MDNS`        |

```yaml
enableMDNS: true
```

### `statusServerPort`

The port for the Iknite status HTTPS server.

| Property | Value                       |
| -------- | --------------------------- |
| Type     | `int`                       |
| Default  | `11443`                     |
| CLI flag | `--status-server-port`      |
| Env var  | `IKNITE_STATUS_SERVER_PORT` |

```yaml
statusServerPort: 11443
```

## Example Configuration Files

### WSL2 Development Environment

```yaml
# /etc/iknite.d/iknite.yaml
domainName: "iknite.local"
ip: "192.168.99.2"
createIp: true
networkInterface: "eth0"
clusterName: "iknite"
enableMDNS: true
useEtcd: false
kustomization: "/etc/iknite.d"
```

### VM / Cloud Instance (No IP management)

```yaml
# /etc/iknite.d/iknite.yaml
domainName: "" # Use actual IP
createIp: false # Don't create secondary IP
enableMDNS: false # No mDNS needed
clusterName: "my-cluster"
useEtcd: false
```

### etcd Backend

```yaml
# /etc/iknite.d/iknite.yaml
useEtcd: true
apiBackendDatabaseDirectory: "/var/lib/etcd"
```

### Custom Kustomization

```yaml
# /etc/iknite.d/iknite.yaml
kustomization: "/etc/my-custom-iknite.d"
```

## Environment Variables

All options can be set via environment variables:

```bash
export IKNITE_DOMAIN_NAME="my-cluster.local"
export IKNITE_IP="10.0.0.100"
export IKNITE_CREATE_IP="false"
export IKNITE_CLUSTER_NAME="my-cluster"
```

## Service Configuration

The OpenRC service configuration is in `/etc/conf.d/iknite`:

```bash
# /etc/conf.d/iknite
IKNITE_ARGS="start -t 120"
```

To pass configuration via the service:

```bash
# /etc/conf.d/iknite
IKNITE_ARGS="start -t 120 --domain-name my-cluster.local"
```

Or use environment variable overrides in the service file:

```bash
# /etc/conf.d/iknite
IKNITE_DOMAIN_NAME="my-cluster.local"
IKNITE_CREATE_IP="false"
IKNITE_ARGS="start -t 120"
```

<!-- cSpell: words addresspool overlayfs softlevel runlevels runlevel tmpfs -->

!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Directories

This page explains the directory structure used by Iknite, Kubernetes
components, containerd, and related tools.

## Iknite Directories

### `/etc/iknite.d/`

The main configuration directory for Iknite, installed by the APK package.

```
/etc/iknite.d/
├── iknite.yaml           ← Iknite configuration file (optional)
└── base/
    ├── kustomization.yaml  ← Bootstrap kustomization
    ├── coredns.yaml        ← CoreDNS manifest
    ├── kube-flannel.yaml   ← Flannel CNI manifest
    └── kube-vip-addresspool.yaml ← Kube-VIP address pool
```

The `kustomization.yaml` in the root also references external resources
(metrics-server, local-path-provisioner, kube-vip-cloud-provider).

### `/etc/conf.d/iknite`

OpenRC service configuration:

```bash
# /etc/conf.d/iknite
IKNITE_ARGS="start -t 120"
```

### `/etc/init.d/iknite`

OpenRC init script that starts/stops the `iknite init` process.

### `/run/iknite/`

Runtime state directory (volatile, cleared on reboot):

| File          | Purpose                                    |
| ------------- | ------------------------------------------ |
| `status.json` | Current cluster state (`IkniteCluster` CR) |

## Kubernetes Directories

### `/etc/kubernetes/`

Kubernetes configuration directory, managed by kubeadm:

```
/etc/kubernetes/
├── admin.conf              ← Admin kubeconfig
├── kubelet.conf            ← kubelet kubeconfig
├── controller-manager.conf ← Controller manager kubeconfig
├── scheduler.conf          ← Scheduler kubeconfig
├── iknite.conf             ← Iknite status server client kubeconfig
├── manifests/
│   ├── kube-apiserver.yaml     ← API server static pod
│   ├── kube-controller-manager.yaml
│   ├── kube-scheduler.yaml
│   ├── kube-vip.yaml           ← Kube-VIP static pod
│   └── kine.yaml               ← kine static pod (or etcd.yaml)
└── pki/
    ├── ca.crt / ca.key         ← Kubernetes CA
    ├── apiserver.crt / apiserver.key
    ├── apiserver-kubelet-client.crt / .key
    ├── apiserver-etcd-client.crt / .key
    ├── front-proxy-ca.crt / .key
    ├── front-proxy-client.crt / .key
    ├── sa.pub / sa.key         ← Service account tokens
    ├── iknite-server.crt / .key ← Status server certificate
    ├── iknite-client.crt / .key ← Status server client certificate
    └── etcd/                   ← etcd certificates (if using etcd)
        ├── ca.crt / ca.key
        ├── server.crt / server.key
        ├── peer.crt / peer.key
        └── healthcheck-client.crt / .key
```

### `/root/.kube/`

User kubeconfig directory:

| File          | Purpose                                |
| ------------- | -------------------------------------- |
| `config`      | Admin kubeconfig (copy of admin.conf)  |
| `iknite.conf` | Iknite status server client kubeconfig |

### `/var/lib/kubelet/`

Kubelet data directory:

```
/var/lib/kubelet/
├── config.yaml          ← Kubelet configuration
├── kubeadm-flags.env    ← kubeadm-generated kubelet flags
├── pki/                 ← Kubelet TLS certificates
└── pods/                ← Pod data (volumes, etc.)
```

## API Backend Directories

### `/var/lib/kine/` (default)

Kine database directory:

| File      | Purpose                                     |
| --------- | ------------------------------------------- |
| `kine.db` | SQLite database with all Kubernetes objects |

### `/var/lib/etcd/` (when `useEtcd: true`)

etcd data directory:

```
/var/lib/etcd/
└── member/
    ├── snap/
    │   └── db          ← etcd snapshot database
    └── wal/            ← Write-ahead log
```

## containerd Directories

### `/var/lib/containerd/`

Main containerd data directory:

```
/var/lib/containerd/
├── io.containerd.content.v1.content/  ← Image layer blobs
├── io.containerd.snapshotter.v1.overlayfs/ ← Container filesystems
├── io.containerd.metadata.v1.bolt/    ← containerd metadata
└── io.containerd.runtime.v2.task/     ← Running container tasks
```

!!! note "Disk usage" This directory can grow large over time. Use
`nerdctl image prune -a` to clean up unused images.

### `/run/containerd/`

Runtime state (volatile):

| Path              | Purpose                |
| ----------------- | ---------------------- |
| `containerd.sock` | containerd gRPC socket |
| `containerd.pid`  | containerd PID file    |

### `/etc/containerd/`

containerd configuration:

| File          | Purpose                  |
| ------------- | ------------------------ |
| `config.toml` | containerd configuration |

### `/etc/crictl.yaml`

crictl (CRI client) configuration:

```yaml
runtime-endpoint: unix:///run/containerd/containerd.sock
image-endpoint: unix:///run/containerd/containerd.sock
timeout: 30
debug: false
```

## Local Path Provisioner Directories

### `/opt/local-path-provisioner/`

Default storage root for PersistentVolumes:

```
/opt/local-path-provisioner/
└── <namespace>-<pvc-name>-<pv-name>/  ← PersistentVolume data
    └── <data files>
```

!!! warning "Persistence" This directory persists across cluster restarts but is
**not** backed up automatically. Consider setting up regular backups.

## BuildKit Directories

### `/var/lib/buildkit/`

BuildKit cache and metadata directory.

### `/run/buildkit/`

Runtime state:

| Path             | Purpose              |
| ---------------- | -------------------- |
| `buildkitd.sock` | BuildKit gRPC socket |

## OpenRC Directories

### `/run/openrc/`

OpenRC runtime state:

| Path        | Purpose                                      |
| ----------- | -------------------------------------------- |
| `softlevel` | Marks OpenRC as initialized (file existence) |

### `/etc/runlevels/`

Runlevel configuration:

```
/etc/runlevels/
└── default/
    ├── iknite -> /etc/init.d/iknite
    ├── containerd -> /etc/init.d/containerd
    └── buildkitd -> /etc/init.d/buildkitd
```

### `/etc/rc.conf`

OpenRC global configuration. Iknite patches this to prevent kubelet from
starting as an OpenRC service:

```bash
# Added by iknite
rc_keyword="-start" kubelet
```

## Summary Table

| Directory                      | Purpose                   | Persistent? |
| ------------------------------ | ------------------------- | ----------- |
| `/etc/iknite.d/`               | Iknite configuration      | Yes         |
| `/run/iknite/`                 | Runtime state             | No (tmpfs)  |
| `/etc/kubernetes/`             | Kubernetes config & certs | Yes         |
| `/var/lib/kubelet/`            | Kubelet data              | Yes         |
| `/var/lib/kine/`               | Kine database             | Yes         |
| `/var/lib/etcd/`               | etcd data                 | Yes         |
| `/var/lib/containerd/`         | Container images & data   | Yes         |
| `/run/containerd/`             | containerd runtime        | No (tmpfs)  |
| `/opt/local-path-provisioner/` | PV storage                | Yes         |
| `/var/lib/buildkit/`           | BuildKit cache            | Yes         |

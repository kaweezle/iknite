# Optionally replace etcd with Kine

<!-- cSpell: words livez readyz healthz seccomp txlock -->

## Motivation

etcd is a critical component of Kubernetes, but it can be complex to set up and
maintain. Kine is a lightweight alternative to etcd that can be used in certain
scenarios, such as development or testing environments. By allowing users to
optionally replace etcd with Kine, we can provide a more flexible and
user-friendly experience for those who do not require the full capabilities of
etcd.

## Implementation

The default behavior will be to use kine instead of etcd, but the user will be
able to opt out of this behavior and use etcd instead if they prefer. To achieve
this, they can set the `--use-etcd` flag to `true` when running the `init`
command.

a `UseEtcd` bool flag will be added to the IkniteClusterSpec struct in
[types.go](../../../pkg/apis/iknite/v1alpha1/types.go#L30). This flag will
indicate whether to use etcd.

in [init.go](../../../pkg/cmd/init.go#L250), We currently initialize etcd with:

```go
initRunner.AppendPhase(WrapPhase(phases.NewEtcdPhase(), ikniteApi.Initializing, nil))
```

`phases.NewEtcdPhase()` is present in
[this file](/root/go/pkg/mod/k8s.io/kubernetes@v1.35.1/cmd/kubeadm/app/cmd/phases/init/etcd.go)

It creates a Pod spec in `/etc/kubernetes/manifests/etcd.yaml` via
`CreateLocalEtcdStaticPodManifestFile`in
[this file](/root/go/pkg/mod/k8s.io/kubernetes@v1.35.1/cmd/kubeadm/app/phases/etcd/local.go).

In the case Kine is used, we will skip the etcd phase and instead create a Kine
Pod spec in `/etc/kubernetes/manifests/kine.yaml`. This will be done by adding a
new phase for Kine and conditionally executing it based on the value of the
`UseEtcd` flag.

The `NewKubeVipControlPlanePhase` function in
[pkg/k8s/phases/init/kube_vip.go](../../../pkg/k8s/phases/init/kube_vip.go#L18)
Creates a Pod spec for kube-vip in `/etc/kubernetes/manifests/kube-vip.yaml`.

We will create a similar function `NewKineControlPlanePhase` that creates a Pod
spec for Kine in `/etc/kubernetes/manifests/kine.yaml`. This function will be
called in the initialization process if the `UseEtcd` flag is set to `false`.

The `NewKubeVipControlPlanePhase` uses a template. `NewKineControlPlanePhase`
can do the same.

The current pod for etcd looks like this:

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    kubeadm.kubernetes.io/etcd.advertise-client-urls: https://192.168.99.2:2379
  labels:
    component: etcd
    tier: control-plane
  name: etcd
  namespace: kube-system
spec:
  containers:
    - command:
        - etcd
        - --advertise-client-urls=https://192.168.99.2:2379
        - --cert-file=/etc/kubernetes/pki/etcd/server.crt
        - --client-cert-auth=true
        - --data-dir=/var/lib/etcd
        - --feature-gates=InitialCorruptCheck=true
        - --initial-advertise-peer-urls=https://192.168.99.2:2380
        - --initial-cluster=amg16=https://192.168.99.2:2380
        - --key-file=/etc/kubernetes/pki/etcd/server.key
        - --listen-client-urls=https://127.0.0.1:2379,https://192.168.99.2:2379
        - --listen-metrics-urls=http://127.0.0.1:2381
        - --listen-peer-urls=https://192.168.99.2:2380
        - --name=amg16
        - --peer-cert-file=/etc/kubernetes/pki/etcd/peer.crt
        - --peer-client-cert-auth=true
        - --peer-key-file=/etc/kubernetes/pki/etcd/peer.key
        - --peer-trusted-ca-file=/etc/kubernetes/pki/etcd/ca.crt
        - --snapshot-count=10000
        - --trusted-ca-file=/etc/kubernetes/pki/etcd/ca.crt
        - --watch-progress-notify-interval=5s
      image: registry.k8s.io/etcd:3.6.5-0
      imagePullPolicy: IfNotPresent
      livenessProbe:
        failureThreshold: 8
        httpGet:
          host: 127.0.0.1
          path: /livez
          port: probe-port
          scheme: HTTP
        initialDelaySeconds: 10
        periodSeconds: 10
        timeoutSeconds: 15
      name: etcd
      ports:
        - containerPort: 2381
          name: probe-port
          protocol: TCP
      readinessProbe:
        failureThreshold: 3
        httpGet:
          host: 127.0.0.1
          path: /readyz
          port: probe-port
          scheme: HTTP
        periodSeconds: 1
        timeoutSeconds: 15
      resources:
        requests:
          cpu: 100m
          memory: 100Mi
      startupProbe:
        failureThreshold: 24
        httpGet:
          host: 127.0.0.1
          path: /readyz
          port: probe-port
          scheme: HTTP
        initialDelaySeconds: 10
        periodSeconds: 10
        timeoutSeconds: 15
      volumeMounts:
        - mountPath: /var/lib/etcd
          name: etcd-data
        - mountPath: /etc/kubernetes/pki/etcd
          name: etcd-certs
  hostNetwork: true
  priority: 2000001000
  priorityClassName: system-node-critical
  securityContext:
    seccompProfile:
      type: RuntimeDefault
  volumes:
    - hostPath:
        path: /etc/kubernetes/pki/etcd
        type: DirectoryOrCreate
      name: etcd-certs
    - hostPath:
        path: /var/lib/etcd
        type: DirectoryOrCreate
      name: etcd-data
status: {}
```

The pod spec for Kine needs to look something like this:

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    kubeadm.kubernetes.io/etcd.advertise-client-urls: https://192.168.99.2:2379
  labels:
    component: kine
    tier: control-plane
  name: kine
  namespace: kube-system
spec:
  containers:
    - command:
        - kine
        - --endpoint=sqlite:///var/lib/kine/kine.db?_journal=WAL&cache=shared&_busy_timeout=30000&_txlock=immediate
        - --trusted-ca-file=/etc/kubernetes/pki/etcd/ca.crt
        - --server-cert-file=/etc/kubernetes/pki/etcd/server.crt
        - --server-key-file=/etc/kubernetes/pki/etcd/server.key
        - --listen-addr=0.0.0.0:2379
        - --metrics-bind-address=:2381
      image: ghcr.io/k3s-io/kine:v0.14.12
      imagePullPolicy: IfNotPresent
      name: kine
      ports:
        - containerPort: 2379
          name: etcd-port
          protocol: TCP
      resources:
        requests:
          cpu: 100m
          memory: 100Mi
      volumeMounts:
        - mountPath: /var/lib/kine
          name: kine-data
        - mountPath: /etc/kubernetes/pki/etcd
          name: kine-certs
  hostNetwork: true
  priority: 2000001000
  priorityClassName: system-node-critical
  securityContext:
    seccompProfile:
      type: RuntimeDefault
  volumes:
    - hostPath:
        path: /etc/kubernetes/pki/etcd
        type: DirectoryOrCreate
      name: kine-certs
    - hostPath:
        path: /var/lib/kine
        type: DirectoryOrCreate
      name: kine-data
status: {}
```

## Tests

To test this feature, we will need to verify that the Kine pod template is
correctly rendered and that the Kine pod is created and running when the
`UseEtcd` flag is set to `false`. We will also need to verify that the etcd pod
is created and running when the `UseEtcd` flag is set to `true`.

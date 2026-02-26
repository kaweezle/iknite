package init

// cSpell: disable

const kineManifestTemplate = `
apiVersion: v1
kind: Pod
metadata:
  annotations:
    kubeadm.kubernetes.io/etcd.advertise-client-urls: https://{{ .Ip }}:2379
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
        - --listen-address=0.0.0.0:2379
        - --metrics-bind-address=:2381
      image: {{ KineImage }}
      imagePullPolicy: IfNotPresent
      name: kine
      ports:
        - containerPort: 2379
          name: etcd-port
          protocol: TCP
        - containerPort: 2381
          name: metrics-port
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
      securityContext:
        allowPrivilegeEscalation: true
  hostNetwork: true
  priority: 2000001000
  priorityClassName: system-node-critical
  securityContext:
    runAsUser: 0
    runAsGroup: 0
    fsGroup: 0
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
`

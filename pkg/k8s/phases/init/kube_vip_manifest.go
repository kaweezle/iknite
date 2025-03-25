package init

// cSpell: disable

const kubeVipManifestTemplate = `
apiVersion: v1
kind: Pod
metadata:
  name: kube-vip
  namespace: kube-system
spec:
  containers:
    - args:
        - manager
      env:
        - name: vip_arp
          value: "true"
        - name: port
          value: "6443"
        - name: vip_nodename
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: vip_interface
          value: eth0
        - name: vip_cidr
          value: "24"
        - name: dns_mode
          value: first
        - name: cp_enable
          value: "true"
        - name: cp_namespace
          value: kube-system
        - name: svc_enable
          value: "true"
        - name: svc_leasename
          value: plndr-svcs-lock
        - name: vip_leaderelection
          value: "true"
        - name: vip_leasename
          value: plndr-cp-lock
        - name: vip_leaseduration
          value: "5"
        - name: vip_renewdeadline
          value: "3"
        - name: vip_retryperiod
          value: "1"
        - name: address
          value: {{ .Ip }}
        - name: prometheus_server
          value: :2112
      image: ghcr.io/kube-vip/kube-vip:v0.8.9
      imagePullPolicy: IfNotPresent
      name: kube-vip
      resources: {}
      securityContext:
        capabilities:
          add:
            - NET_ADMIN
            - NET_RAW
      volumeMounts:
        - mountPath: /etc/kubernetes/admin.conf
          name: kubeconfig
  hostAliases:
    - hostnames:
        - kubernetes
        {{- if .DomainName }}
        - {{ .DomainName }}
        {{- end }}
      ip: 127.0.0.1
  hostNetwork: true
  volumes:
    - hostPath:
        path: /etc/kubernetes/admin.conf
      name: kubeconfig
`

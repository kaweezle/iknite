#!/bin/sh

ip=$(kubectl cluster-info | head -1 | sed -E '1 s#.*https://(.*):.*#\1#g')
cat - <<EOF
kind: ResourceList
items:
- apiVersion: metallb.io/v1beta1
  kind: IPAddressPool
  metadata:
    name: node-ip-pool
    namespace: metallb-system
    annotations:
      argocd.argoproj.io/sync-wave: "1"
  spec:
    addresses:
    - $ip/32
- apiVersion: metallb.io/v1beta1
  kind: L2Advertisement
  metadata:
    name: advertisement
    namespace: metallb-system
    annotations:
      argocd.argoproj.io/sync-wave: "1"
EOF

#!/bin/sh

[ $(id -u) -eq 0 ] || exec doas $0 $@

set -x

# /sbin/openrc boot

/etc/init.d/kubelet stop
/etc/init.d/crio stop
/etc/init.d/iknite-config stop
/etc/init.d/iknite-init stop

pschildren() {
    ps -e -o ppid= -o pid= | \
    sed -e 's/^\s*//g; s/\s\s*/\t/g;' | \
    grep -w "^$1" | \
    cut -f2
}

pstree() {
    for pid in $@; do
        echo $pid
        for child in $(pschildren $pid); do
            pstree $child
        done
    done
}

killtree() {
    kill -9 $(
        { set +x; } 2>/dev/null;
        pstree $@;
        set -x;
    ) 2>/dev/null
}

getshims() {
    ps -e -o pid= -o args= | sed -e 's/^ *//; s/\s\s*/\t/;' | grep -w '/usr/b[i]n/conmon' | cut -f1
}

killtree $({ set +x; } 2>/dev/null; getshims; set -x)

do_unmount_and_remove() {
    set +x
    while read -r _ path _; do
        case "$path" in $1*) echo "$path" ;; esac
    done < /proc/self/mounts | sort -r | xargs -r -t -n 1 sh -c 'umount "$0" && rm -rf "$0"'
    set -x
}

do_unmount() {
    set +x
    while read -r _ path _; do
        case "$path" in $1*) echo "$path" ;; esac
    done < /proc/self/mounts | sort -r | xargs -r -t -n 1 sh -c 'umount "$0"'
    set -x
}

do_unmount '/var/lib/containers/storage'
do_unmount '/var/lib/kubelet/pods'
do_unmount '/var/lib/kubelet/plugins'
do_unmount_and_remove '/run/containers/storage'
do_unmount_and_remove '/run/netns'
do_unmount_and_remove '/run/ipcns'
do_unmount_and_remove '/run/utsns'

# Remove CNI namespaces
ip netns show 2>/dev/null | xargs -r -t -n 1 ip netns delete

# Delete network interface(s) that match 'master cni0'
ip link show 2>/dev/null | grep 'master cni0' | while read ignore iface ignore; do
    iface=${iface%%@*}
    [ -z "$iface" ] || ip link delete $iface
done

ip link delete cni0
ip link delete flannel.1

# rm -rf /var/lib/cni/

iptables-save | grep -v KUBE- | grep -v CNI- | grep -iv flannel | iptables-restore
ip6tables-save | grep -v KUBE- | grep -v CNI- | grep -iv flannel | ip6tables-restore

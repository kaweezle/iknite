#!/bin/sh
# cSpell: disable
# set -e: exit on error
set -e
_step_counter=0
step() {
	_step_counter=$(( _step_counter + 1 ))
	printf '\n\033[1;36m%d) %s\033[0m\n' $_step_counter "$@" >&2  # bold cyan
}

# Adds specified services to the runlevel. Current working directory must be
# root of the image.
rc_add() {
	local runlevel="$1"; shift  # runlevel name
	local services="$*"  # names of services

	local svc; for svc in $services; do
		mkdir -p "/etc/runlevels/$runlevel"
		if [ -f "/etc/runlevels/$runlevel/$svc" ]; then
            echo " * service $svc already in runlevel $runlevel"
        else
            ln -s "/etc/init.d/$svc" "/etc/runlevels/$runlevel/$svc"
		    echo  " * service $svc added to runlevel $runlevel"
        fi
	done
}

step 'Add services packages...'
apk add --quiet --no-cache --no-progress less logrotate openssh chrony cloud-init dhcpcd doas acpi e2fsprogs-extra py3-pyserial py3-netifaces alpine-base

step 'Set up timezone'
setup-timezone -z Europe/Paris

step 'Set up keymap'
setup-keymap fr fr-azerty

step 'Restore default rc.conf'
cp /etc/rc.conf.orig /etc/rc.conf
echo "" >> /etc/rc.conf
# shellcheck disable=SC2140
echo "rc_kubelet_need="non-existing-service"" >> /etc/rc.conf

step 'Set up networking'
cat > /etc/network/interfaces <<-EOF
	auto lo
	iface lo inet loopback

	auto eth0
	iface eth0 inet manual

	post-up /etc/network/if-post-up.d/*
	post-down /etc/network/if-post-down.d/*
EOF

mkdir -p etc/network/if-post-up.d
mkdir -p etc/network/if-post-down.d

step "Setting resolv.conf"
cat > /etc/resolv.conf <<-EOF
# Default nameservers, replace them with your own.
nameserver 1.1.1.1
nameserver 2606:4700:4700::1111
EOF

# FIXME: remove root and alpine password
step "Set cloud configuration"
sed -e '/disable_root:/ s/true/false/' \
	-e '/ssh_pwauth:/ s/0/no/' \
    -e '/name: alpine/a \    passwd: "*"' \
    -e '/lock_passwd:/ s/True/False/' \
    -e '/shell:/ s#/bin/ash#/bin/zsh#' \
    -i /etc/cloud/cloud.cfg

step 'Allow only key based ssh login'
sed -e '/PermitRootLogin yes/d' \
    -e 's/^#PasswordAuthentication yes/PasswordAuthentication no/' \
    -e 's/^#PubkeyAuthentication yes/PubkeyAuthentication yes/' \
    -i /etc/ssh/sshd_config

# Terraform and github actions need ssh-rsa as accepted algorithm
# The ssh client needs to be updated (see https://www.openssh.com/txt/release-8.8)
echo "PubkeyAcceptedKeyTypes=+ssh-rsa" >> /etc/ssh/sshd_config

step 'Remove password for users'
usermod -p '*' root

step 'Adjust rc.conf'
sed -Ei \
	-e 's/^[# ](rc_depend_strict)=.*/\1=NO/' \
	-e 's/^[# ](rc_logger)=.*/\1=YES/' \
	-e 's/^[# ](unicode)=.*/\1=YES/' \
	/etc/rc.conf


# see https://gitlab.alpinelinux.org/alpine/aports/-/issues/88²61
step 'Enable cloud-init configuration via NoCloud iso image'

echo "iso9660" >> /etc/filesystems

step 'Enable sysinit services'
rc_add sysinit devfs dmesg mdev hwdrivers
[ -e /etc/init.d/cgroups ] && rc_add sysinit cgroups ||:  # since v3.8

step 'Enable boot services'
rc_add boot modules hwclock swap hostname sysctl bootmisc syslog cloud-init-local
# urandom was renamed to seedrng in v3.17
if [ -e /etc/init.d/seedrng ]; then
    rc_add boot seedrng
else
    rc_add boot urandom
fi

step 'Enable shutdown services'
rc_add shutdown killprocs savecache mount-ro

step 'Enable defaultservices'
rc_add default acpid chronyd crond dhcpcd networking termencoding sshd cloud-init-ds-identify cloud-init cloud-config cloud-final iknite

step 'Clean up apk cache'
if [ -L /etc/apk/cache ]; then
	rm /etc/apk/cache >/dev/null 2>&1
fi
rm -Rf /var/cache/apk/* ||:

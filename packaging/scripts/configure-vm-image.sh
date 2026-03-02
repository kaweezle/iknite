#!/bin/sh
# cSpell: words secureboot runlevel runlevels chrony makestep hwclock azerty uefi mkkeys nvram virt bootx mkinitfs
# cSpell: words resolv dhcpcd allowinterfaces pwauth sysinit seedrng devfs urandom killprocs acpid crond chronyd
# cSpell:words termencoding defaultservices bootmisc iface hwdrivers mdev blkid doas acpi fsprogs pyserial netifaces
# cSpell: words vmbus storvsc
# set -e: exit on error
set -e

# Root filesystem UUID passed from build-vm-image.sh as first argument.
# Needed to set the correct kernel cmdline in secureboot.conf.
ROOT_UUID="${1:-}"
if [ -z "$ROOT_UUID" ]; then
    # Fallback: discover uuid from /proc/mounts when running standalone
    _root_dev=$(awk '$2 == "/" {print $1}' /proc/mounts | head -1)
    ROOT_UUID=$(blkid -s UUID -o value "$_root_dev" 2>/dev/null || echo "")
fi

_step_counter=0
step() {
	_step_counter=$(( _step_counter + 1 ))
	printf '\n\033[1;36m%d) %s\033[0m\n' $_step_counter "$@" >&2  # bold cyan
}

info() {
	printf '\n\033[0;36m==> %s\033[0m\n' "$@" >&2  # cyan
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
echo "" >> /etc/chrony/chrony.conf
echo "makestep 1 3" >> /etc/chrony/chrony.conf
sed -e 's/clock="UTC"/clock="local"/' -i /etc/conf.d/hwclock

step 'Set up keymap'
setup-keymap fr fr-azerty

step 'Restore default rc.conf'
cp /etc/rc.conf.orig /etc/rc.conf
echo "" >> /etc/rc.conf
# shellcheck disable=SC2140
echo "rc_kubelet_need="non-existing-service"" >> /etc/rc.conf

step 'Configure UEFI Unified Kernel Image'
# Create the EFI boot directory on the mounted EFI partition (/efi).
install -d -m 755 /efi/EFI/Boot

# Generate self-signed UEFI keys required by secureboot-hook to sign the UKI.
# The VM can boot with secure boot disabled; the keys are still needed to
# produce a valid signed binary that UEFI firmware can load.
apk add --quiet --no-cache --no-progress efi-mkkeys
install -d -m 700 /etc/uefi-keys
efi-mkkeys -s "Iknite VM" -o /etc/uefi-keys
apk del --quiet efi-mkkeys
# The binary mount command is installed by efi-mkkeys. When removed, we need to restore the /bin/mount symlink.
if [ ! -f /bin/mount ]; then
    ln -s /bin/busybox /bin/mount
fi

# Configure secureboot-hook to generate the UKI with the correct kernel
# command line and place it at the default UEFI bootloader path so that
# no NVRAM boot entry is needed (works on Hyper-V and other UEFI firmware).
cat > /etc/kernel-hooks.d/secureboot.conf <<-EOF
# Kernel command line for the Unified Kernel Image.
# Note: do NOT include initrd= here; the UKI bundles initramfs internally.
cmdline="root=UUID=${ROOT_UUID} modules=ext4,hv_vmbus,hv_storvsc quiet console=tty0 console=ttyS0,115200n8"

# For the virt kernel, install the UKI into the default UEFI path.
if [ "\$1" == "virt" ]; then
  output_dir="/efi/EFI/Boot"
  output_name="bootx64.efi"
fi
EOF

info "Generating Unified Kernel Image via kernel-hooks (apk fix kernel-hooks)"
apk fix kernel-hooks

# Disable the mkinitfs trigger: the UKI bundles the initramfs internally,
# so a separate mkinitfs run on kernel upgrade is not needed.
if ! grep -q 'disable_trigger' /etc/mkinitfs/mkinitfs.conf 2>/dev/null; then
    echo 'disable_trigger=yes' >> /etc/mkinitfs/mkinitfs.conf
fi

step 'Set up networking'
cat > /etc/network/interfaces <<-EOF
	auto lo
	iface lo inet loopback

	auto eth0
	iface eth0 inet manual

	post-up /etc/network/if-post-up.d/*
	post-down /etc/network/if-post-down.d/*
EOF

mkdir -p /etc/network/if-post-up.d
mkdir -p /etc/network/if-post-down.d

step "Setting resolv.conf"
cat > /etc/resolv.conf <<-EOF
# Default nameservers, replace them with your own.
nameserver 1.1.1.1
nameserver 2606:4700:4700::1111
EOF

step 'Configuring dhcpcd'
echo "allowinterfaces eth*" >> /etc/dhcpcd.conf
echo "ipv4only" >> /etc/dhcpcd.conf

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

#!/usr/bin/env sh
# cSpell: words nocloud genisoimage volid cidata subformat qcow2 cdrkit nodiscard blockdev getsize writeback blkid fsprogs progname wgets
# cSpell: words mountpoint resolv resolvconf runlevel runlevels hotplug udevadm mdev extlinux virt mkinitfs virtio syslinux relatime vhdx
# cSpell: words inittab securetty gsub
set -e

# Step names for dynamic --skip-* and --only-* handling
STEP_NAMES="create-image mount-image copy-rootfs install-kernel install-bootloader configure-vm cleanup build-vhdx build-iso"
ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)

# Only run this specific step (empty means run all non-skipped steps)
ONLY_CALLED=false
IMAGE_SIZE="3G"
SERIAL_PORT="ttyS0"
KUBERNETES_VERSION=${KUBERNETES_VERSION:-$(grep k8s.io/kubernetes "$ROOT_DIR/go.mod" | awk '{gsub(/^v/,"",$2);print $2;}')}
if [ -z "$IKNITE_VERSION" ]; then
    IKNITE_VERSION=$(jq -Mr ".version" dist/metadata.json)
fi
readonly PROGNAME='build-vm-image'
HOST_ARCH="$(uname -m)"
readonly HOST_ARCH

# SHA256 checksum of $APK_TOOLS_URI for each architecture.
case "$HOST_ARCH" in
	aarch64) : "${APK_TOOLS_SHA256:="811783d95de35845c4bcbcfaa27c94d711c286fdf4c0edde51dcb06ea532eab5"}";;
	x86_64) : "${APK_TOOLS_SHA256:="87f9f360dd1aeed03b9ab18f0dd24e6edf73f5f4de1092ab9d1e2ecaf47e8ba9"}";;
esac

: "${APK_TOOLS_URI:="https://gitlab.alpinelinux.org/api/v4/projects/5/packages/generic/v2.14.9/$HOST_ARCH/apk.static"}"
: "${APK:="apk"}"
: "${APK_OPTS:="--no-progress"}"

if ! command -v realpath >/dev/null; then
	alias realpath='readlink -f'
fi

_apk() {
    # shellcheck disable=SC2086
	"$APK" $APK_OPTS "$@"
}

_step_counter=0
step() {
	_step_counter=$(( _step_counter + 1 ))
	printf '\n\033[1;36m%d) %s\033[0m\n' $_step_counter "$@" >&2  # bold cyan
}

skip() {
	_step_counter=$(( _step_counter + 1 ))
	printf '\n\033[1;35m%d) %s (Skipped)\033[0m\n' $_step_counter "$@" >&2  # bold magenta
}

error() {
    printf '\n\033[1;31mError: %s\033[0m\n' "$@" >&2  # bold red
}

info() {
    printf '\n\033[0;36m==> %s\033[0m\n' "$@" >&2  # cyan
}

warning() {
    printf '\n\033[1;33mWarning: %s\033[0m\n' "$@" >&2  # bold yellow
}

skip_all() {
    if [ "$ONLY_CALLED" != false ]; then
        return
    fi
    for s in $STEP_NAMES; do
        eval "SKIP_$(step_to_var "$s")=true"
    done
}

# Check if a step name is valid
is_valid_step() {
    for s in $STEP_NAMES; do
        if [ "$s" = "$1" ]; then
            return 0
        fi
    done
    return 1
}

# Convert step name to variable name (e.g., add-images -> ADD_IMAGES)
step_to_var() {
    echo "$1" | tr '[:lower:]-' '[:upper:]_'
}

# Check if a step should run
should_run_step() {
    local step_name="$1"
    local skip_var
    skip_var="SKIP_$(step_to_var "$step_name")"

    # Otherwise, check the skip flag
    eval "local skip_value=\$$skip_var"
    # shellcheck disable=SC2154
    if [ "$skip_value" = "true" ]; then
        return 1
    fi
    local done_var
    done_var="DONE_$(step_to_var "$step_name")"
    eval "local done_value=\$$done_var"
    # shellcheck disable=SC2154
    if [ "$done_value" = "true" ]; then
        return 1
    fi
    return 0
}

# Prints path of available nbdX device, or returns 1 if not any.
get_available_nbd() {
	local dev
    # shellcheck disable=SC2044
    for dev in $(find /dev -maxdepth 2 -name 'nbd[0-9]*'); do
		if [ "$(blockdev --getsize64 "$dev")" -eq 0 ]; then
			echo "$dev"; return 0
		fi
	done
	return 1
}

# Attaches the specified image as a NBD block device and prints its path.
attach_image() {
	local image="$1"
	local format="${2:-}"
	local disk_dev

	disk_dev=$(get_available_nbd) || {
		modprobe nbd max_part=16
		sleep 1
		disk_dev=$(get_available_nbd)
	} || { error 'No available nbd device found!'; exit 1; }

	qemu-nbd --connect="$disk_dev" --cache=writeback \
		${format:+--format=$format} "$image" || return 1

	sleep 1  # see #45
	echo "$disk_dev"
}

# Prints UUID of filesystem on the specified block device.
blk_uuid() {
	local dev="$1"
	blkid "$dev" | sed -En 's/.*\bUUID="([^"]+)".*/\1/p'
}

# Binds the directory $1 at the mountpoint $2 and sets propagation to private.
mount_bind() {
	mkdir -p "$2"
	mount --bind "$1" "$2"
	mount --make-private "$2"
}

# Prepares chroot at the specified path.
prepare_chroot() {
	local dest="$1"

	mkdir -p "$dest"/proc
	mount -t proc none "$dest"/proc
	mount_bind /dev "$dest"/dev
	mount_bind /sys "$dest"/sys

	install -D -m 644 /etc/resolv.conf "$dest"/etc/resolv.conf
	echo "$RESOLVCONF_MARK" >> "$dest"/etc/resolv.conf
}

# Adds specified services to the runlevel. Current working directory must be
# root of the image.
rc_add() {
	local runlevel="$1"; shift  # runlevel name
	local services="$*"  # names of services

	local svc; for svc in $services; do
		mkdir -p "etc/runlevels/$runlevel"
		ln -s "/etc/init.d/$svc" "etc/runlevels/$runlevel/$svc"
		echo " * service $svc added to runlevel $runlevel"
	done
}

# Ensures that the specified device node exists.
settle_dev_node() {
	local dev="$1"

	[ -e "$dev" ] && return 0

	sleep 1  # give hotplug handler some time to kick in
	[ -e "$dev" ] && return 0

	if has_cmd udevadm; then
		udevadm settle --exit-if-exists="$dev"
	elif has_cmd mdev; then
		mdev -s
	fi
	[ -e "$dev" ] && return 0

	return 1
}

# Installs and configures extlinux.
setup_extlinux() {
	local mnt="$1"  # path of directory where is root device currently mounted
	local root_dev="$2"  # root device
	local modules="$3"  # modules which should be loaded before pivot_root
	local kernel_flavor="$4"  # name of default kernel to boot
	local serial_port="$5"  # serial port number for serial console
	local default_kernel="$kernel_flavor"
	local kernel_opts=''

	[ -z "$serial_port" ] || kernel_opts="console=tty0 console=$serial_port,115200n8"

	if [ "$kernel_flavor" = 'virt' ]; then
		_apk search --root . --exact --quiet linux-lts | grep -q . \
			&& default_kernel='lts' \
			|| default_kernel='vanilla'
	fi

	sed -Ei \
		-e "s|^[# ]*(root)=.*|\1=$root_dev|" \
		-e "s|^[# ]*(default_kernel_opts)=.*|\1=\"$kernel_opts\"|" \
		-e "s|^[# ]*(modules)=.*|\1=\"$modules\"|" \
		-e "s|^[# ]*(default)=.*|\1=$default_kernel|" \
		-e "s|^[# ]*(serial_port)=.*|\1=$serial_port|" \
		"$mnt"/etc/update-extlinux.conf

	chroot "$mnt" extlinux --install /boot
	chroot "$mnt" update-extlinux --warn-only 2>&1 \
		| { grep -Fv 'extlinux: cannot open device /dev' ||:; } >&2
}

# Configures mkinitfs.
setup_mkinitfs() {
	local mnt="$1"  # path of directory where is root device currently mounted
	local features="$2"  # list of mkinitfs features

	features=$(printf '%s\n' "$features" | sort | uniq | xargs)

	sed -Ei "s|^[# ]*(features)=.*|\1=\"$features\"|" \
		"$mnt"/etc/mkinitfs/mkinitfs.conf
}

# Unmounts all filesystem under the specified directory tree.
umount_recursively() {
	local mount_point="$1"
	test -n "$mount_point" || return 1

	cat /proc/mounts \
		| cut -d ' ' -f 2 \
		| grep "^$mount_point" \
		| sort -r \
		| xargs umount -rn
}

# Downloads the specified file using wget and checks checksum.
wgets() (
	local url="$1"
	local sha256="$2"
	local dest="${3:-.}"

	cd "$dest" \
		&& wget -T 10 --no-verbose "$url" \
		&& echo "$sha256  ${url##*/}" | sha256sum -c
)


cleanup() {
    trap '' EXIT HUP INT TERM  # unset trap to avoid loop
    if should_run_step "cleanup"; then
        step "Cleaning"

        # if mounted, unmount
        if [ -n "$mount_dir" ]; then
            info "Unmounting filesystems under $mount_dir"
            umount_recursively "$mount_dir"
        fi
        rmdir "$mount_dir" || warning "Failed to remove mount directory $mount_dir"
        if [ -n "$disk_dev" ] && ! [ -b "$IMAGE_FILE" ]; then
            info "Detaching NBD device $disk_dev"
            sync
            sleep 1
            qemu-nbd --disconnect "$disk_dev"
        else
            warning "No NBD device to detach."
        fi
        # Remove temporary directory for APK tools if created
        if [ -d "$temp_dir" ]; then
            rm -Rf "$temp_dir"
        fi
        eval "DONE_$(step_to_var "cleanup")=true"
    else
        skip "Cleanup"
    fi
}


usage() {
    cat << EOF
Usage: $(basename "$0") [OPTIONS]

Build Iknite VM image with configurable steps.

OPTIONS:
    -h, --help          Show this help message and exit
    --skip-<step>       Skip a specific step
    --only-<step>       Run only the specified step (skip all others)
    --size <size>       Set the VM image size (default: $IMAGE_SIZE)

STEPS:
    create-image       Create the VM image
    mount-image        Mount the VM image
    copy-rootfs        Copy the root filesystem into the VM image
    install-kernel     Install the kernel into the VM image
    install-bootloader Install the bootloader into the VM image
    configure-vm       Configure the VM (networking, ssh, etc.)
    unmount-image      Unmount the VM image
    cleanup            Clean up temporary resources
    build-vhdx         Build VHDX image
    build-iso          Build ISO image
EOF
}


# Parse command-line arguments
while [ $# -gt 0 ]; do
    case "$1" in
        -h|--help)
            usage
            exit 0
            ;;
        --skip-*)
            step_name="${1#--skip-}"
            if is_valid_step "$step_name"; then
                eval "SKIP_$(step_to_var "$step_name")=true"
            else
                error "Unknown step: $step_name"
                usage
                exit 1
            fi
            shift
            ;;
        --only-*)
            step_name="${1#--only-}"
            if is_valid_step "$step_name"; then
                skip_all
                ONLY_CALLED=true
                eval "SKIP_$(step_to_var "$step_name")=false"
            else
                error "Unknown step: $step_name"
                usage
                exit 1
            fi
            shift
            ;;
        *)
            error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

trap cleanup EXIT HUP INT TERM


# Alpine packages required for building VM image
# qemu-img cdrkit e2fsprogs

NEEDED_COMMANDS="qemu-img qemu-nbd genisoimage"
for cmd in $NEEDED_COMMANDS; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        error "Required command not found: $cmd"
        exit 1
    fi
done

# rm -rf build
# mkdir -p build
IMAGE_FORMAT="qcow2"
IMAGE_FILE="dist/iknite-vm.${IKNITE_VERSION}-${KUBERNETES_VERSION}.${IMAGE_FORMAT}"

CLOUD_CONFIG_FILE=${1:-cloud-config.yaml}
info "Creating iknite VM image $IMAGE_FILE using cloud config file: $CLOUD_CONFIG_FILE"


if should_run_step "create-image"; then
    step "Creating VM image..."

    if [ -f "$IMAGE_FILE" ]; then
        warning "Image file $IMAGE_FILE already exists. Skipping creation."
    else
        qemu-img create -f $IMAGE_FORMAT "$IMAGE_FILE" $IMAGE_SIZE
    fi

    if pgrep -f "qemu-nbd.*$IMAGE_FILE" >/dev/null 2>&1; then
        warning "Image $IMAGE_FILE is already attached as a NBD device."
        disk_dev=$(pgrep -f "qemu-nbd.*$IMAGE_FILE" | xargs -I{} readlink /proc/{}/fd/13)
    else
        info "Attaching image $IMAGE_FILE as a NBD device"
        disk_dev=$(attach_image "$IMAGE_FILE" "$IMAGE_FORMAT")
    fi
    info "Image attached as $disk_dev"

    # Check that the filesystem does not already exist
    if blkid "$disk_dev" | grep -q UUID >/dev/null 2>&1; then
        warning "Block device $disk_dev already has a filesystem. Skipping creation."
    else
        info "Creating ext4 filesystem on $disk_dev"
        mkfs.ext4 -O ^64bit -E nodiscard "$disk_dev" >/dev/null || {
            error "Failed to create ext4 filesystem on $disk_dev"
            exit 1
        }
    fi
    eval "DONE_$(step_to_var "create-image")=true"
else
    skip "Creating VM image"
fi


if should_run_step "mount-image"; then
    step "Mounting VM image..."
    root_uuid=$(blk_uuid "$disk_dev")
    mount_dir=$(mktemp -d /tmp/$PROGNAME.XXXXXX)

    info "Mounting root filesystem (UUID: $root_uuid) at $mount_dir"
    mount -o rw,defaults UUID="$root_uuid" "$mount_dir" || {
        error "Failed to mount root filesystem UUID=$root_uuid at $mount_dir"
        exit 1
    }
    eval "DONE_$(step_to_var "mount-image")=true"
else
    skip "Mounting VM image"
fi

if should_run_step "copy-rootfs"; then
    step "Copying root filesystem to VM image..."

    if [ -d "$mount_dir/root" ] && [ -f "$mount_dir/root/.rootfs.sha256sum" ] && sha256sum -c "$mount_dir/root/.rootfs.sha256sum" >/dev/null 2>&1; then
        warning "Root filesystem already exists in $mount_dir. Skipping copy."
    else
        info "Extracting root filesystem to $mount_dir"
        tar -C "$mount_dir" -xpf "dist/iknite-${IKNITE_VERSION}-${KUBERNETES_VERSION}.rootfs.tar.gz" || {
            error "Failed to extract root filesystem to $mount_dir"
            exit 1
        }
        sha256sum "dist/iknite-${IKNITE_VERSION}-${KUBERNETES_VERSION}.rootfs.tar.gz" > "$mount_dir/root/.rootfs.sha256sum"
    fi
    eval "DONE_$(step_to_var "copy-rootfs")=true"
else
    skip "Copying root filesystem to VM image"
fi


temp_dir=''
if ! command -v "$APK" >/dev/null; then
	info "$APK not found, downloading static apk-tools"

	temp_dir="$(mktemp -d /tmp/$PROGNAME.XXXXXX)"
	wgets "$APK_TOOLS_URI" "$APK_TOOLS_SHA256" "$temp_dir"
	APK="$temp_dir/apk.static"
	chmod +x "$APK"
fi

prepare_chroot "$mount_dir"

if should_run_step "install-kernel"; then
    step "Installing kernel to VM image..."

    if [ -f "$mount_dir/boot/vmlinuz-linux" ]; then
        warning "Kernel already exists in $mount_dir/boot. Skipping installation."
    else
        info "Installing kernel to $mount_dir/boot"
        _apk add --root "$mount_dir" linux-virt || {
            error "Failed install kernel to $mount_dir"
            exit 1
        }
    fi
    eval "DONE_$(step_to_var "install-kernel")=true"
else
    skip "Installing kernel to VM image"
fi

if should_run_step "install-bootloader"; then
    step "Installing bootloader to VM image..."

    _apk add --root "$mount_dir" mkinitfs || {
        error "Failed install mkinitfs to $mount_dir"
        exit 1
    }

    info "Setting up mkinitfs"
    setup_mkinitfs "$mount_dir" "base ext4 kms scsi virtio"

    info "Setting up extlinux bootloader"
    _apk add --root "$mount_dir" --no-scripts syslinux
    setup_extlinux "$mount_dir" "UUID=$root_uuid" "ext4" "virt" "$SERIAL_PORT"

    eval "DONE_$(step_to_var "install-bootloader")=true"
else
    skip "Installing bootloader to VM image"
fi


if should_run_step "configure-vm"; then
    step "Configuring VM image..."

    info "Setting up fstab"
    cat > "$mount_dir/etc/fstab" <<-EOF
# <fs>		<mountpoint>	<type>	<opts>		<dump/pass>
UUID=$root_uuid	/		ext4	relatime	0 1
EOF

    info "Setting up serial console"
	echo "$SERIAL_PORT" >> "$mount_dir/etc/securetty"
	sed -Ei "s|^[# ]*($SERIAL_PORT:.*)|\1|" "$mount_dir/etc/inittab"


    script_dir=$(dirname "$(realpath "$0")")
    info "Executing script in chroot: $script_dir/configure-vm-image.sh"
    mount_bind "${script_dir}" "$mount_dir/mnt"
    chroot "$mount_dir" /bin/sh -c "cd /mnt && ./configure-vm-image.sh" || {
        error "Script $script_dir/configure-vm-image.sh failed in chroot"
        exit 1
    }
    eval "DONE_$(step_to_var "configure-vm")=true"
else
    skip "Configuring VM image"
fi

cleanup

if should_run_step "build-vhdx"; then
    step "Building VHDX image from $IMAGE_FILE..."

    VHDX_IMAGE_FILE="dist/iknite-vm.${IKNITE_VERSION}-${KUBERNETES_VERSION}.vhdx"
    qemu-img convert "$IMAGE_FILE" -O vhdx -o subformat=dynamic "$VHDX_IMAGE_FILE"

    eval "DONE_$(step_to_var "build-vhdx")=true"
else
    skip "Building VHDX image"
fi

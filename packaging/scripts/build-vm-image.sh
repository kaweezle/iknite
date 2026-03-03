#!/usr/bin/env sh
# cSpell: words nocloud volid cidata subformat qcow2 cdrkit nodiscard blkid losetup notrunc
# cSpell: words mountpoint resolv resolvconf runlevel runlevels hotplug udevadm mdev extlinux virt mkinitfs virtio
# cSpell: words inittab securetty gsub toplevel uefi efi sfdisk dosfstools efistub secureboot ukifile vfat mkdosfs mkfat
# cSpell: words fsprogs progname wgets syslinux relatime vhdx bootable noatime fmask iocharset bootx tarcmd bsdtar
set -e

# Step names for dynamic --skip-* and --only-* handling
STEP_NAMES="create-image mount-image copy-rootfs install-kernel install-bootloader configure-vm cleanup build-qcow2 build-vhdx build-iso"
# TODO:try git rev-parse --show-toplevel
ROOT_DIR=$(cd "$(dirname "$0")/../../" && pwd)


# Only run this specific step (empty means run all non-skipped steps)
ONLY_CALLED=false
IMAGE_SIZE="3G"
SERIAL_PORT="ttyS0"
KUBERNETES_VERSION=${KUBERNETES_VERSION:-$(grep k8s.io/kubernetes "$ROOT_DIR/go.mod" | awk '{gsub(/^v/,"",$2);print $2;}')}
if [ -z "$IKNITE_VERSION" ]; then
    IKNITE_VERSION=$(jq -Mr ".version" "$ROOT_DIR/dist/metadata.json")
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

TARCMD="tar"
if command -v bsdtar >/dev/null; then
    echo "Using bsdtar for better compatibility with tar archives"
	TARCMD="bsdtar"
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

# Attaches the specified raw image as a loop device and prints its path.
attach_loop() {
	local img="$1"
	losetup --find --show "$img"
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

# Tests if the specified command exists on the system.
has_cmd() {
	command -v "$1" >/dev/null
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

# Creates a GPT partition table for UEFI boot on the given device.
# Partition layout:
#   p1 - EFI System Partition (400M, FAT32, bootable)
#   p2 - root partition (remaining space, ext4)
create_gpt() {
	local dev="$1"
	printf '%s\n' \
		'label: gpt' \
		'name=efi,type=U,size=400M,bootable' \
		'name=system,type=L' \
		| sfdisk "$dev"
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
        rmdir "$mount_dir" 2>/dev/null || warning "Failed to remove mount directory $mount_dir"
        # Detach loop devices
        sync
        sleep 1
        if [ -n "$efi_loop" ]; then
            info "Detaching EFI loop device $efi_loop"
            losetup -d "$efi_loop" || warning "Failed to detach EFI loop device $efi_loop"
            efi_loop=''
        else
            warning "No EFI loop device to detach."
        fi
        if [ -n "$root_loop" ]; then
            info "Detaching root loop device $root_loop"
            losetup -d "$root_loop" || warning "Failed to detach root loop device $root_loop"
            root_loop=''
        else
            warning "No root loop device to detach."
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
    build-qcow2        Assemble partition images into raw disk and convert to qcow2
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
# qemu-img cdrkit e2fsprogs sfdisk dosfstools

NEEDED_COMMANDS="qemu-img sfdisk losetup"
for cmd in $NEEDED_COMMANDS; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
        error "Required command not found: $cmd"
        exit 1
    fi
done

# dosfstools provides mkfs.fat (mkdosfs)
if ! command -v mkfs.fat >/dev/null 2>&1 && ! command -v mkdosfs >/dev/null 2>&1; then
    error "Required command not found: mkfs.fat (install dosfstools)"
    exit 1
fi
MKFS_FAT=$(command -v mkfs.fat 2>/dev/null || command -v mkdosfs)

IMAGE_FORMAT="qcow2"
IMAGE_DIR="$ROOT_DIR/dist/images"
mkdir -p "$IMAGE_DIR"
IMAGE_FILE="$IMAGE_DIR/iknite-vm.${IKNITE_VERSION}-${KUBERNETES_VERSION}.${IMAGE_FORMAT}"
RAW_IMAGE_FILE="$IMAGE_DIR/iknite-vm.${IKNITE_VERSION}-${KUBERNETES_VERSION}.raw"
EFI_IMG="$IMAGE_DIR/efi-${IKNITE_VERSION}-${KUBERNETES_VERSION}.raw"
ROOT_IMG="$IMAGE_DIR/root-${IKNITE_VERSION}-${KUBERNETES_VERSION}.raw"

CLOUD_CONFIG_FILE=${1:-cloud-config.yaml}
info "Creating iknite VM image $IMAGE_FILE using cloud config file: $CLOUD_CONFIG_FILE"


if should_run_step "create-image"; then
    step "Creating VM image partition files..."

    # Create raw disk image with GPT partition table to determine partition layout
    if [ -f "$RAW_IMAGE_FILE" ]; then
        warning "Raw image file $RAW_IMAGE_FILE already exists. Skipping creation."
    else
        info "Creating raw disk image $RAW_IMAGE_FILE ($IMAGE_SIZE)"
        truncate -s "$IMAGE_SIZE" "$RAW_IMAGE_FILE"
        info "Creating GPT partition table on $RAW_IMAGE_FILE"
        create_gpt "$RAW_IMAGE_FILE"
    fi

    # Read partition layout from the GPT raw image (sectors of 512 bytes each)
    efi_sectors=$(sfdisk --dump "$RAW_IMAGE_FILE" | awk -F'size=' '/name="efi"/{split($2,a,","); gsub(/[^0-9]/,"",a[1]); print a[1]}')
    root_sectors=$(sfdisk --dump "$RAW_IMAGE_FILE" | awk -F'size=' '/name="system"/{split($2,a,","); gsub(/[^0-9]/,"",a[1]); print a[1]}')

    if [ -z "$efi_sectors" ] || [ -z "$root_sectors" ]; then
        error "Failed to read partition sizes from GPT layout in $RAW_IMAGE_FILE"
        exit 1
    fi

    efi_bytes=$((efi_sectors * 512))
    root_bytes=$((root_sectors * 512))

    # Create raw partition images sized exactly to match the GPT layout
    if [ ! -f "$EFI_IMG" ]; then
        info "Creating EFI partition image $EFI_IMG ($efi_bytes bytes)"
        truncate -s "$efi_bytes" "$EFI_IMG"
    fi
    if [ ! -f "$ROOT_IMG" ]; then
        info "Creating root partition image $ROOT_IMG ($root_bytes bytes)"
        truncate -s "$root_bytes" "$ROOT_IMG"
    fi

    # Attach partition images as loop devices
    if losetup -j "$EFI_IMG" 2>/dev/null | grep -q "$EFI_IMG"; then
        efi_loop=$(losetup -j "$EFI_IMG" | awk -F: '{print $1}')
        warning "EFI image $EFI_IMG is already attached as $efi_loop"
    else
        efi_loop=$(attach_loop "$EFI_IMG") || { error "Failed to attach $EFI_IMG as a loop device"; exit 1; }
        info "EFI partition image attached as $efi_loop"
    fi

    if losetup -j "$ROOT_IMG" 2>/dev/null | grep -q "$ROOT_IMG"; then
        root_loop=$(losetup -j "$ROOT_IMG" | awk -F: '{print $1}')
        warning "Root image $ROOT_IMG is already attached as $root_loop"
    else
        root_loop=$(attach_loop "$ROOT_IMG") || { error "Failed to attach $ROOT_IMG as a loop device"; exit 1; }
        info "Root partition image attached as $root_loop"
    fi

    # Format EFI partition as FAT32
    if blkid "$efi_loop" 2>/dev/null | grep -q 'TYPE="vfat"'; then
        warning "EFI partition $efi_loop already formatted as FAT32. Skipping."
    else
        info "Formatting EFI partition $efi_loop as FAT32 ($efi_bytes bytes)"
        "$MKFS_FAT" -F32 -n EFI "$efi_loop" || {
            error "Failed to format EFI partition $efi_loop as FAT32"
            exit 1
        }
    fi

    # Format root partition as ext4
    if blkid "$root_loop" 2>/dev/null | grep -q 'TYPE="ext4"'; then
        warning "Root partition $root_loop already has a filesystem. Skipping."
    else
        info "Creating ext4 filesystem on root partition $root_loop"
        mkfs.ext4 -O ^64bit -E nodiscard "$root_loop" >/dev/null || {
            error "Failed to create ext4 filesystem on $root_loop"
            exit 1
        }
    fi
    eval "DONE_$(step_to_var "create-image")=true"
else
    skip "Creating VM image partition files"
fi


if should_run_step "mount-image"; then
    step "Mounting VM partition images..."
    root_uuid=$(blk_uuid "$root_loop")
    mount_dir=$(mktemp -d /tmp/$PROGNAME.XXXXXX)

    info "Mounting root filesystem (UUID: $root_uuid) at $mount_dir"
    mount -o rw,defaults UUID="$root_uuid" "$mount_dir" || {
        error "Failed to mount root filesystem UUID=$root_uuid at $mount_dir"
        exit 1
    }

    info "Mounting EFI partition at $mount_dir/efi"
    mkdir -p "$mount_dir/efi"
    mount -t vfat "$efi_loop" "$mount_dir/efi" || {
        error "Failed to mount EFI partition $efi_loop at $mount_dir/efi"
        exit 1
    }
    eval "DONE_$(step_to_var "mount-image")=true"
else
    skip "Mounting VM partition images"
fi

if should_run_step "copy-rootfs"; then
    step "Copying root filesystem to VM image..."

    if [ -d "$mount_dir/root" ] && [ -f "$mount_dir/root/.rootfs.sha256sum" ] && sha256sum -c "$mount_dir/root/.rootfs.sha256sum" >/dev/null 2>&1; then
        warning "Root filesystem already exists in $mount_dir. Skipping copy."
    else
        info "Extracting root filesystem to $mount_dir"
        $TARCMD -C "$mount_dir" -xpf "$ROOT_DIR/dist/iknite-${IKNITE_VERSION}-${KUBERNETES_VERSION}.rootfs.tar.gz" || {
            error "Failed to extract root filesystem to $mount_dir"
            exit 1
        }
        sha256sum "$ROOT_DIR/dist/iknite-${IKNITE_VERSION}-${KUBERNETES_VERSION}.rootfs.tar.gz" > "$mount_dir/root/.rootfs.sha256sum"
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

    info "Installing UEFI boot packages (mkinitfs, systemd-efistub, secureboot-hook)"
    _apk add --root "$mount_dir" mkinitfs systemd-efistub secureboot-hook || {
        error "Failed to install UEFI boot packages to $mount_dir"
        exit 1
    }

    info "Setting up mkinitfs"
    setup_mkinitfs "$mount_dir" "base ext4 kms scsi virtio"

    if [ -f "$mount_dir/boot/vmlinuz-virt" ]; then
        warning "Kernel already exists in $mount_dir/boot. Skipping installation."
    else
        info "Installing kernel linux-virt to $mount_dir"
        _apk add --root "$mount_dir" linux-virt || {
            error "Failed to install kernel to $mount_dir"
            exit 1
        }
    fi
    eval "DONE_$(step_to_var "install-kernel")=true"
else
    skip "Installing kernel to VM image"
fi

if should_run_step "install-bootloader"; then
    step "Setting up UEFI boot directory structure..."

    # For UEFI boot, we use a Unified Kernel Image (UKI) placed at the default
    # UEFI bootloader path /efi/EFI/Boot/bootx64.efi. The UKI is generated by
    # secureboot-hook via 'apk fix kernel-hooks' which runs in configure-vm.
    info "Creating EFI boot directory structure"
    mkdir -p "$mount_dir/efi/EFI/Boot"

    eval "DONE_$(step_to_var "install-bootloader")=true"
else
    skip "Setting up UEFI boot directory structure"
fi


if should_run_step "configure-vm"; then
    step "Configuring VM image..."

    info "Setting up fstab"
    cat > "$mount_dir/etc/fstab" <<-EOF
# <fs>           <mountpoint>  <type>  <opts>                                                                                     <dump/pass>
UUID=$root_uuid  /             ext4    relatime                                                                                    0 1
LABEL=EFI        /efi          vfat    rw,noatime,fmask=0133,codepage=437,iocharset=ascii,shortname=mixed,utf8,errors=remount-ro  0 2
EOF

    info "Setting up serial console"
	echo "$SERIAL_PORT" >> "$mount_dir/etc/securetty"
	sed -Ei "s|^[# ]*($SERIAL_PORT:.*)|\1|" "$mount_dir/etc/inittab"


    script_dir=$(dirname "$(realpath "$0")")
    info "Executing script in chroot: $script_dir/configure-vm-image.sh"
    mount_bind "${script_dir}" "$mount_dir/mnt"
    chroot "$mount_dir" /bin/sh -c "cd /mnt && ./configure-vm-image.sh '$root_uuid'" || {
        error "Script $script_dir/configure-vm-image.sh failed in chroot"
        exit 1
    }
    eval "DONE_$(step_to_var "configure-vm")=true"
else
    skip "Configuring VM image"
fi

cleanup

if should_run_step "build-qcow2"; then
    step "Assembling and converting VM image to qcow2..."

    if [ -f "$IMAGE_FILE" ]; then
        warning "Image file $IMAGE_FILE already exists. Skipping conversion."
    else
        # Read partition offsets from the GPT raw image (partition data written at these sector offsets)
        efi_start=$(sfdisk --dump "$RAW_IMAGE_FILE" | awk -F'start=' '/name="efi"/{split($2,a,","); gsub(/[^0-9]/,"",a[1]); print a[1]}')
        root_start=$(sfdisk --dump "$RAW_IMAGE_FILE" | awk -F'start=' '/name="system"/{split($2,a,","); gsub(/[^0-9]/,"",a[1]); print a[1]}')

        if [ -z "$efi_start" ] || [ -z "$root_start" ]; then
            error "Failed to read partition offsets from GPT layout in $RAW_IMAGE_FILE"
            exit 1
        fi

        info "Merging EFI partition image into raw disk at sector $efi_start"
        dd if="$EFI_IMG" of="$RAW_IMAGE_FILE" bs=512 seek="$efi_start" conv=notrunc || {
            error "Failed to merge EFI partition image into raw disk"
            exit 1
        }

        info "Merging root partition image into raw disk at sector $root_start"
        dd if="$ROOT_IMG" of="$RAW_IMAGE_FILE" bs=512 seek="$root_start" conv=notrunc || {
            error "Failed to merge root partition image into raw disk"
            exit 1
        }

        info "Converting raw disk image to $IMAGE_FORMAT: $IMAGE_FILE"
        qemu-img convert -f raw -O $IMAGE_FORMAT "$RAW_IMAGE_FILE" "$IMAGE_FILE" || {
            error "Failed to convert raw disk image to $IMAGE_FORMAT"
            exit 1
        }

        info "Removing intermediate partition image files"
        rm -f "$EFI_IMG" "$ROOT_IMG" "$RAW_IMAGE_FILE"
    fi

    eval "DONE_$(step_to_var "build-qcow2")=true"
else
    skip "Assembling and converting VM image to qcow2"
fi

if should_run_step "build-vhdx"; then
    step "Building VHDX image from $IMAGE_FILE..."

    VHDX_IMAGE_FILE="$IMAGE_DIR/iknite-vm.${IKNITE_VERSION}-${KUBERNETES_VERSION}.vhdx"
    qemu-img convert "$IMAGE_FILE" -O vhdx -o subformat=dynamic "$VHDX_IMAGE_FILE"

    eval "DONE_$(step_to_var "build-vhdx")=true"
else
    skip "Building VHDX image"
fi

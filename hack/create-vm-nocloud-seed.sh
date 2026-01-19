#!/usr/bin/env sh
# cSpell: disable
set -eu

usage() {
    cat <<EOF
Usage: $(basename "$0") [-h|--help] <destination.iso> <file1> [file2 ...]

Create an ISO image with the specified files.

Arguments:
    destination.iso    Output ISO filename
    file1 [file2 ...]  Files to include in the ISO

Options:
    -h, --help         Show this help message

Example:
    $(basename "$0") seed.iso user-data meta-data
EOF
}

# Check for help flag
if [ $# -eq 0 ] || [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    usage
    exit 0
fi

# Check if genisoimage is available
if ! command -v genisoimage >/dev/null 2>&1; then
    echo "Error: genisoimage command not found" >&2
    echo "Please install genisoimage (or cdrtools/xorriso)" >&2
    exit 1
fi

# Check minimum arguments
if [ $# -lt 2 ]; then
    echo "Error: Insufficient arguments" >&2
    echo "" >&2
    usage
    exit 1
fi

# Get destination filename
DEST_ISO="$1"
shift

# Check if all source files exist
for file in "$@"; do
    if [ ! -f "$file" ]; then
        echo "Error: File not found: $file" >&2
        exit 1
    fi
done

# Create ISO
genisoimage -output "$DEST_ISO" -volid CIDATA -joliet -rock "$@"

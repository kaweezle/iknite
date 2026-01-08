#!/bin/sh
# Script to extract APK signing key from SOPS encrypted secrets file
# and install it in the project root directory

set -eu

# Define paths
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
SECRETS_FILE="${PROJECT_ROOT}/support/iac/iknite/secrets.sops.yaml"

# Check if sops is available
if ! command -v sops > /dev/null 2>&1; then
    echo "Error: sops command not found. Please install sops first." >&2
    exit 1
fi

# Check if python3 is available
if ! command -v python3 > /dev/null 2>&1; then
    echo "Error: python3 command not found. Please install python3 first." >&2
    exit 1
fi

# Check if secrets file exists
if [ ! -f "${SECRETS_FILE}" ]; then
    echo "Error: Secrets file not found at ${SECRETS_FILE}" >&2
    exit 1
fi

echo "Extracting signing key from ${SECRETS_FILE}..."

# Use Python to parse the JSON output from sops and extract the signing key
sops -d --output-type json "${SECRETS_FILE}" | python3 -c "
import sys
import os
import json

try:
    data = json.load(sys.stdin)

    if 'apk_signing_key' not in data:
        print('Error: apk_signing_key not found in secrets file', file=sys.stderr)
        sys.exit(1)

    apk_key = data['apk_signing_key']

    if 'name' not in apk_key:
        print('Error: apk_signing_key.name not found in secrets file', file=sys.stderr)
        sys.exit(1)

    if 'private_key' not in apk_key:
        print('Error: apk_signing_key.private_key not found in secrets file', file=sys.stderr)
        sys.exit(1)

    key_name = apk_key['name']
    private_key = apk_key['private_key']

    # Define output file path
    output_file = os.path.join('${PROJECT_ROOT}', f'{key_name}.rsa')

    # Write the private key to file
    with open(output_file, 'w') as f:
        f.write(private_key)

    # Set file permissions to be readable only by the user (400)
    os.chmod(output_file, 0o400)

    print(f'Successfully created signing key file: {output_file}')
    print(f'File permissions: {oct(os.stat(output_file).st_mode)[-3:]}')

except json.JSONDecodeError as e:
    print(f'Error parsing JSON: {e}', file=sys.stderr)
    sys.exit(1)
except Exception as e:
    print(f'Error: {e}', file=sys.stderr)
    sys.exit(1)
"

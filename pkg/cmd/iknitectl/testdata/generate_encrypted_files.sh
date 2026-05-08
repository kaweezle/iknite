#!/bin/bash

CMD_DIR=$(cd "$(dirname "$0")" && pwd)

config_file=$(mktemp)
trap 'rm -f "$config_file"' EXIT
cat >"$config_file" <<EOF
creation_rules:
  - encrypted_regex: ^data\$
EOF

for f in "$CMD_DIR"/*_plain.yaml; do
    base_name=$(basename "$f" _plain.yaml)
    sops --config "$config_file" -e -a 'age1mjrhxft836jdjm6jem37ue788za2ngk6xaegayst0thf9amc55uqzxtn87' "$f" >"$CMD_DIR/${base_name}_encrypted.yaml"
done

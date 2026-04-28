Create an `init` subcommand for iknitectl secrets to create the
`secrets.sops.yaml` file with the appropriate structure and encrypted values.

The command will create the following files:

- a `.sops.yaml` file with the appropriate configuration for SOPS
- a `secrets.sops.yaml` file with the appropriate structure and encrypted values
- a `~/.ssh/id_ed25519` private key and `~/.ssh/id_ed25519.pub` public key pair
  if it doesn't already exist

If either the `.sops.yaml` or `secrets.sops.yaml` files already exist, the
command will not overwrite them and will instead print a message indicating that
they already exist.

The `.sops.yaml` file will have the following content:

```yaml
creation_rules:
  - path_regex: .*\.sops\.yaml$
    encrypted_regex: "^data$"
    # This is the ~/.ssh/id_ed25519.pub public key, but you can replace it with your own public key
    age: >-
      ssh-ed25519 <public_key> <comment>
stores:
  json:
    indent: 2
  json_binary:
    indent: 2
  yaml:
    indent: 2
```

The `secrets.sops.yaml` file will have the following content:

```yaml
# cspell: disable
apiVersion: config.karmafun.dev/v1alpha1
kind: SopsGenerator
metadata:
  name: iknite-secrets
  annotations:
    config.kaweezle.com/local-config: "true"
    config.kubernetes.io/function: |
      exec:
        path: karmafun
data:
  secrets:
    #  ~/.ssh/id_ed25519.pub
    public_key: &ed25519_public_key ssh-ed25519 <public_key> <comment>
    #  ~/.ssh/id_ed25519
    private_key: &ed25519_private_key |
      <private_key>
```

The command will use the `age` encryption method with the generated SSH key pair
to encrypt the secrets in the `secrets.sops.yaml` file.

The command will take the following optional flags:

- `--force` (-f): to overwrite the existing files if they already exist.
- `--key-file` (-k): to specify a custom SSH key file. If the file does not
  exist, it will be generated. In this case a message will be printed inviting
  the use the set the environment variable `SOPS_AGE_SSH_PRIVATE_KEY_FILE` to
  the path of the generated private key file.

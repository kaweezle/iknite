#cloud-config
#cSpell:disable

# Fixed SSH host keys ensure the VM always presents the same host key,
# allowing clients to verify the host key against a pre-configured known_hosts file.
# This eliminates the need for StrictHostKeyChecking=no in SSH clients.
ssh_keys:
  ed25519_private: |
    ${ssh_host_ed25519_private}
  ed25519_public: "${ssh_host_ed25519_public}"

#!/bin/sh
# cSpell: words libstdc skopeo tenv doas socat rootlesskit slirp4netns buildkit buildctl nerdctl goreleaser signingkey gpgsign sopsdiffer textconv

_step_counter=0
step() {
	_step_counter=$(( _step_counter + 1 ))
	printf '\n\033[1;36m%d) %s\033[0m\n' $_step_counter "$@" >&2  # bold cyan
}

echo "Setting up development environment..."
ARG TFLINT_VERSION=0.60.0
ARG TERRAFORM_DOCS_VERSION=0.21.0


step "Installing packages..."
# shellcheck disable=SC2046
apk update --quiet && \
apk add --quiet --no-progress --no-cache zsh tzdata git libstdc++ doas iproute2 gnupg socat openssh openrc curl tar zstd && \
apk add --quiet --no-progress --no-cache pipx uv go jq skopeo tenv kubectl k9s golangci-lint gnupg sops age nodejs npm openssl abuild && \
apk add --quiet --no-progress --no-cache ip6tables containerd kubelet kubeadm cni-plugins cni-plugin-flannel util-linux-misc buildkit buildctl nerdctl rootlesskit slirp4netns && \
GORELEASER_VERSION=$(curl --silent  https://api.github.com/repos/goreleaser/goreleaser/releases/latest | jq -r .tag_name | sed -e 's/^v//') && \
wget -q -O /tmp/goreleaser.apk "https://github.com/goreleaser/goreleaser/releases/download/v${GORELEASER_VERSION}/goreleaser_${GORELEASER_VERSION}_x86_64.apk" && \
apk add --quiet --no-progress --allow-untrusted --no-cache /tmp/goreleaser.apk && \
rm -f /tmp/goreleaser.apk && \
wget -q -O /tmp/tflint.zip "https://github.com/terraform-linters/tflint/releases/download/v${TFLINT_VERSION}/tflint_linux_amd64.zip" && \
unzip -q /tmp/tflint.zip -d /usr/local/bin/ && \
rm -f /tmp/tflint.zip && \
wget -q -O /tmp/terraform-docs.tar.gz "https://github.com/terraform-docs/terraform-docs/releases/download/v${TERRAFORM_DOCS_VERSION}/terraform-docs-v${TERRAFORM_DOCS_VERSION}-linux-amd64.tar.gz" && \
tar -xzf /tmp/terraform-docs.tar.gz -C /usr/local/bin/ terraform-docs && \
rm -f /tmp/terraform-docs.tar.gz && \
rm -rf $(find /var/cache/apk/ -type f)


step "Installing pre-commit..."
pipx install pre-commit

step "Manual steps instructions..."

echo "- Install the age key for SOPS decryption in ~/.config/sops/age/keys.txt"
echo "- To install the signing key, run:"
echo "    go run hack/iknitedev/ install signing-key deploy/iac/iknite/secrets.sops.yaml ."
echo "- Configure git as needed. The following is an example ~/.gitconfig:"
echo ""
printf '\033[90m' # dark gray
cat << 'EOF'
[user]
    signingkey = C1B63969600B4A521810A01DCABBD5CED0F36C61
    name = Antoine Martin
    email = antoine@mrtn.fr
[gpg]
    program = /usr/bin/gpg
[init]
    defaultBranch = main
[commit]
    gpgsign = true
[sequence]
    editor = "code -r --wait"
[core]
    editor = "code -r --wait"
[diff "sopsdiffer"]
    textconv = "sops --decrypt --config /dev/null"
EOF
printf '\033[0m\n' # reset

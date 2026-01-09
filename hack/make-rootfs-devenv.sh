#!/bin/sh
# cSpell: words libstdc skopeo tenv doas socat rootlesskit slirp4netns buildkit buildctl nerdctl goreleaser signingkey gpgsign sopsdiffer textconv

_step_counter=0
step() {
	_step_counter=$(( _step_counter + 1 ))
	printf '\n\033[1;36m%d) %s\033[0m\n' $_step_counter "$@" >&2  # bold cyan
}

echo "Setting up development environment..."

step "Installing packages..."
apk update --quiet && \
apk add --quiet --no-progress --no-cache zsh tzdata git libstdc++ doas iproute2 gnupg socat openssh openrc curl tar zstd && \
apk add --quiet --no-progress --no-cache pipx uv go jq skopeo tenv kubectl k9s golangci-lint gnupg sops age nodejs npm openssl abuild && \
apk add --quiet --no-progress --no-cache ip6tables containerd kubelet kubeadm cni-plugins cni-plugin-flannel util-linux-misc buildkit buildctl nerdctl rootlesskit slirp4netns && \
GORELEASER_VERSION=$(curl --silent  https://api.github.com/repos/goreleaser/goreleaser/releases/latest | jq -r .tag_name | sed -e 's/^v//') && \
wget -q -O /tmp/goreleaser.apk "https://github.com/goreleaser/goreleaser/releases/download/v${GORELEASER_VERSION}/goreleaser_${GORELEASER_VERSION}_x86_64.apk" && \
apk add --quiet --no-progress --allow-untrusted --no-cache /tmp/goreleaser.apk

step "Installing pre-commit..."
pipx install pre-commit

step "Manual steps instructions..."

echo "- Install the age key for SOPS decryption in ~/.config/sops/age/keys.txt"
echo "- Run the script hack/install-signing-key.sh to install the APK signing key"
echo "- Configure git as needed. The following is an example ~/.gitconfig:"
echo ""
echo -e "\033[90m$(cat << 'EOF'
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
)\033[0m"

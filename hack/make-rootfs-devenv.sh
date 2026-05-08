#!/bin/sh
# cSpell: words libstdc skopeo tenv doas socat rootlesskit slirp4netns buildkit buildctl nerdctl goreleaser signingkey
# cSpell: words gpgsign sopsdiffer textconv gojq kubectx

_step_counter=0
step() {
	_step_counter=$(( _step_counter + 1 ))
	printf '\n\033[1;36m%d) %s\033[0m\n' $_step_counter "$@" >&2  # bold cyan
}

echo "Setting up development environment..."


step "Installing packages..."
# shellcheck disable=SC2046
apk update --quiet && \
apk add --quiet --no-progress --no-cache abuild age bash curl doas git gnupg go gojq golangci-lint ip6tables k9s kubectx libarchive-tools make nodejs npm openssl pipx rootlesskit slirp4netns sops uv zstd  && \
test -f /usr/bin/jq || ln -s /usr/bin/gojq /usr/bin/jq && \
AQUA_VERSION=$(curl --silent "https://api.github.com/repos/aquaproj/aqua/releases/latest" | gojq -r .tag_name | sed -e 's/^v//') && \
wget -q -O /tmp/aqua.tar.gz "https://github.com/aquaproj/aqua/releases/download/v${AQUA_VERSION}/aqua_linux_amd64.tar.gz" && \
tar -xzf /tmp/aqua.tar.gz -C /usr/local/bin/ aqua && \
rm -f /tmp/aqua.tar.gz && \
echo "export PATH=\"\${PATH}:$(/usr/local/bin/aqua root-dir)/bin\"" >> /root/.zshrc && \
rm -rf $(find /var/cache/apk/ -type f)


step "Installing pre-commit..."
pipx install pre-commit

step "Manual steps instructions..."

echo "- Install the ssh key for SOPS decryption in ~/.ssh/id_ed25519"
echo "- Add $(aqua root-dir)/bin to your PATH."
echo "- Install go based tools with:"
echo "    aqua i"
echo "- To install the signing key, run:"
echo "    go run hack/iknitectl/iknitectl.go install signing-key secrets.sops.yaml ."
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

#!/bin/sh
# This script sets up and starts an Iknite Kubernetes cluster inside a WSL2 environment.

# Add the Kaweezle APK repository
KEY_URL=$(curl -s https://api.github.com/repos/kaweezle/iknite/releases/latest | grep "browser_download_url.*rsa.pub" | cut -d '"' -f 4 | sed 's/%40/@/g')
wget -q -P /etc/apk/keys "${KEY_URL}"
# Change with https://static.iknite.app/test/ for testing pre-releases
grep -q iknite /etc/apk/repositories || echo https://static.iknite.app/release/ >> /etc/apk/repositories

# Add some minimal dependencies
apk --update add krmfnbuiltin k9s openssl nerdctl

apk add --allow-untrusted --no-cache /mnt/wsl/iknite-*.x86_64.apk

# Start the cluster and deploy the basic components
openrc -n default
iknite -v debug -w 120 start

#!/bin/sh

# Add the Kaweezle APK repository
wget -qO - "https://github.com/kaweezle/iknite/releases/download/v0.1.18/kaweezle-devel@kaweezle.com-c9d89864.rsa.pub" > /etc/apk/keys/kaweezle-devel@kaweezle.com-c9d89864.rsa.pub
grep -q kaweezle /etc/apk/repositories || echo https://kaweezle.com/repo/ >> /etc/apk/repositories 

# Add some minimal dependencies
apk --update add krmfnbuiltin k9s openssl nerdctl

apk add --allow-untrusted --no-cache /mnt/wsl/iknite-*.x86_64.apk

# Start the cluster and deploy the basic components
iknite -v debug -w 120 start

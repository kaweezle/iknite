# syntax=docker/dockerfile:1.3-labs
# cSpell: disable
FROM alpine:3.23 as builder

ARG USERNAME=alpine
ARG GROUPNAME=alpine
ARG TFLINT_VERSION=0.60.0
ARG TERRAFORM_DOCS_VERSION=0.21.0

LABEL org.opencontainers.image.source="https://github.com/kaweezle/iknite.git"

# Add the dependencies
RUN echo "http://dl-cdn.alpinelinux.org/alpine/edge/testing/" >> /etc/apk/repositories && \
    apk update --quiet && \
    apk add --quiet --no-progress --no-cache bash git openssh curl zstd libarchive-tools && \
    apk add --quiet --no-progress --no-cache gojq kubectl kubectx openssl && \
    ln -s /usr/bin/gojq /usr/bin/jq && \
    AQUA_VERSION=$(curl --silent "https://api.github.com/repos/aquaproj/aqua/releases/latest" | gojq -r .tag_name | sed -e 's/^v//') && \
    wget -q -O /tmp/aqua.tar.gz "https://github.com/aquaproj/aqua/releases/download/v${AQUA_VERSION}/aqua_linux_amd64.tar.gz" && \
    tar -xzf /tmp/aqua.tar.gz -C /usr/local/bin/ aqua && \
    rm -f /tmp/aqua.tar.gz && \
    rm -rf `find /var/cache/apk/ -type f` && \
    mkdir -p /workspace && \
    rm -rf `find /var/cache/apk/ -type f` && \
    git config --global --add safe.directory /workspace && \
    mkdir -p /root/.config/aquaproj-aqua

# We disable cosign and SLSA in aqua since they require some disk space
WORKDIR /workspace
ENV PATH="/root/.local/share/aquaproj-aqua/bin:${PATH}" \
    AQUA_DISABLE_COSIGN="true" \
    AQUA_DISABLE_SLSA="true" \
    AQUA_DISABLE_GITHUB_ARTIFACT_ATTESTATION="true" \
    AQUA_GLOBAL_CONFIG="/root/.config/aquaproj-aqua/aqua.yaml"

COPY aqua.yaml /root/.config/aquaproj-aqua/aqua.yaml
RUN /usr/local/bin/aqua install --all && \
    helmfile init --force

CMD /bin/bash

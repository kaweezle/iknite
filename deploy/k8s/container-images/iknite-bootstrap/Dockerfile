# syntax=docker/dockerfile:1.3-labs
# cSpell: disable
FROM alpine:3.23 as builder

LABEL org.opencontainers.image.source="https://github.com/kaweezle/iknite.git"

# Add the dependencies
RUN echo "http://dl-cdn.alpinelinux.org/alpine/edge/testing/" >> /etc/apk/repositories && \
    apk update --quiet && \
    apk add --quiet --no-progress --no-cache bash git openssh curl zstd libarchive-tools && \
    apk add --quiet --no-progress --no-cache gojq kubectl kubectx kustomize helm helmfile sops openssl && \
    ln -s /usr/bin/gojq /usr/bin/jq && \
    rm -rf `find /var/cache/apk/ -type f` && \
    mkdir -p /workspace && \
    rm -rf `find /var/cache/apk/ -type f` && \
    git config --global --add safe.directory /workspace && \
    helmfile init --force && \
    rm -rf /root/.cache/helm/plugins && \
    mkdir -p /root/.ssh && \
    chmod 700 /root/.ssh

COPY kubewait /bin/kubewait
WORKDIR /workspace

CMD /bin/kubewait

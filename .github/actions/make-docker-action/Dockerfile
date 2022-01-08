FROM alpine:3.15

RUN set -euxo pipefail ;\
    echo "http://dl-cdn.alpinelinux.org/alpine/edge/testing/" >> etc/apk/repositories ;\
    apk --no-cache --update add \
        curl \
        git \
        go \
        make \
        skopeo \
        libarchive-tools \
        fuse-overlayfs

COPY entrypoint.sh /entrypoint.sh

ENTRYPOINT [ "/entrypoint.sh" ]

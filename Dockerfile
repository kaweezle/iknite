# To build the image in the build directory, build this docker image:
# docker build -t builder .
# mkdir -p /tmp/containers/storage
# docker run --rm  --privileged -it -v /tmp/containers:/var/lib/containers -v $(pwd):/kaweezle-rootfs builder
FROM alpine:3.15

RUN set -euxo pipefail ;\
    echo "http://dl-cdn.alpinelinux.org/alpine/edge/testing/" >> etc/apk/repositories ;\
    apk --no-cache --update add \
        curl \
        git \
        go \
        make \
        skopeo \
        libarchive-tools

RUN mkdir -p /kaweezle-rootfs/build

WORKDIR /kaweezle-rootfs

CMD [ "make" ]

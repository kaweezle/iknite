# To build the image in the build directory, build this docker image:
# docker build -t builder .
# mkdir -p /tmp/containers/storage
# docker run --rm  --privileged -it -v /tmp/containers:/var/lib/containers -v $(pwd):/k8wsl builder
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

RUN mkdir -p /k8wsl/build

WORKDIR /k8wsl

CMD [ "make" ]

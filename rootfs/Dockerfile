# syntax=docker/dockerfile:1.3-labs
FROM alpine:3.23

ARG IKNITE_VERSION=0.5.3-devel
ARG IKNITE_REPO_URL=https://static.iknite.app/release/
ARG IKNITE_KEY_NAME=kaweezle-devel@kaweezle.com-c9d89864.rsa.pub
ARG KUBERNETES_VERSION=1.35.0

# Add the dependencies
RUN --mount=type=bind,target=/apks  \
    echo "https://dl-cdn.alpinelinux.org/alpine/edge/community/" >> /etc/apk/repositories && \
    echo "https://dl-cdn.alpinelinux.org/alpine/edge/testing/" >> /etc/apk/repositories && \
    echo "${IKNITE_REPO_URL}" >> /etc/apk/repositories && \
    install -m 644 -o root -g root /apks/${IKNITE_KEY_NAME} /etc/apk/keys/${IKNITE_KEY_NAME} && \
    apk update --quiet && \
    apk add --no-progress --no-cache openrc zsh nerdctl openssh && \
    apk add --no-progress --no-cache /apks/iknite-${IKNITE_VERSION}.$(uname -m).apk /apks/krmfnbuiltin-*.$(uname -m).apk && \
    rm -rf `find /var/cache/apk/ -type f`

# Configuration
RUN --mount=type=bind,target=/conf sed -ie '/^root:/ s#:/bin/.*$#:/bin/zsh#' /etc/passwd && \
    touch /etc/subuid && touch /etc/subgid && \
    mkdir -p /lib/rc/init.d && \
    ln -s /lib/rc/init.d /run/openrc && \
    touch /lib/rc/init.d/softlevel && \
    cp /etc/rc.conf /etc/rc.conf.orig && \
    install -m 644 -o root -g root /conf/rc.conf /etc/rc.conf

# Add Oh-my-zsh
RUN --mount=type=bind,target=/conf git clone --quiet --depth 1 https://github.com/ohmyzsh/ohmyzsh.git /usr/share/oh-my-zsh && \
    sed -i -e 's#^export ZSH=.*#export ZSH=/usr/share/oh-my-zsh#g' /usr/share/oh-my-zsh/templates/zshrc.zsh-template && \
    git clone --quiet --depth=1 https://github.com/romkatv/powerlevel10k.git /usr/share/oh-my-zsh/custom/themes/powerlevel10k && \
    git clone --quiet --depth=1  https://github.com/zsh-users/zsh-autosuggestions "/usr/share/oh-my-zsh/custom/plugins/zsh-autosuggestions" && \
    sed -ie '/^plugins=/ s#.*#plugins=(git zsh-autosuggestions)#' /usr/share/oh-my-zsh/templates/zshrc.zsh-template && \
    sed -ie '/^ZSH_THEME=/ s#.*#ZSH_THEME="powerlevel10k/powerlevel10k"#' /usr/share/oh-my-zsh/templates/zshrc.zsh-template && \
    echo '[[ ! -f ~/.p10k.zsh ]] || source ~/.p10k.zsh' >> /usr/share/oh-my-zsh/templates/zshrc.zsh-template && \
    mkdir -p /etc/skel && \
    install -m 700 -o root -g root /usr/share/oh-my-zsh/templates/zshrc.zsh-template /etc/skel/.zshrc  && \
    install -m 600 -o root -g root /conf/p10k.zsh /etc/skel/.p10k.zsh && \
    install --directory -o root -g root -m 0700 /etc/skel/.ssh && \
    cp -a /etc/skel/. /root/

# Configure root user
USER root

# Run shell by default. Allows using the docker image
CMD /bin/zsh

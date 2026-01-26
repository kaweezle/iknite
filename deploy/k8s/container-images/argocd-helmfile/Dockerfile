# cSpell: words ksops awscli zxvf
# Modified version of https://github.com/chatwork/dockerfiles/blob/master/argocd-helmfargoile/Dockerfile
# to include last version of kustomize and krmfnbuiltin, a ksops like.
# The base image can be seen here: https://github.com/argoproj/argo-cd/blob/v3.2.6/Dockerfile
FROM quay.io/argoproj/argocd:v3.2.6

LABEL version="v3.2.6-1.2.3-1"
LABEL maintainer="antoine@mrtn.fr"

# Switch to root for the ability to perform install
USER root

ARG HELMFILE_VERSION=v1.2.3
ARG HELM_VERSION=v3.19.0
ARG HELM_LOCATION="https://get.helm.sh"
ARG HELM_FILENAME="helm-${HELM_VERSION}-linux-amd64.tar.gz"
ARG KUBECTL_VERSION=v1.35.0
ARG SOPS_VERSION=v3.11.0
ARG HELM_DIFF_VERSION=3.14.1
ARG HELM_SECRETS_VERSION=4.7.5
ARG KUSTOMIZE_VERSION=v5.8.0
ARG KRMFNBUILTIN_VERSION=v0.4.3
ARG AGE_VERSION=v1.3.1
ARG VALS_VERSION=0.43.1
ARG TERRAFORM_VERSION=1.14.3
ARG TERRAGRUNT_VERSION=0.97.2

ENV HELM_SECRETS_BACKEND="vals" \
    HELM_SECRETS_HELM_PATH=/usr/local/bin/helm.orig \
    HELM_PLUGINS="/home/argocd/.local/share/helm/plugins/" \
    HELM_SECRETS_VALUES_ALLOW_SYMLINKS=false \
    HELM_SECRETS_VALUES_ALLOW_ABSOLUTE_PATH=false \
    HELM_SECRETS_VALUES_ALLOW_PATH_TRAVERSAL=false \
    HELM_SECRETS_WRAPPER_ENABLED=false

# Install tools needed for your repo-server to retrieve & decrypt secrets, render manifests
# (e.g. curl, awscli, gpg, sops)

# helm
# helmfile, sops, kubectl
RUN set -eux ;\
    apt-get update && \
    apt-get install -y curl gpg apt-utils lsb-release unzip && \
    rm -rf /var/lib/apt/lists/* && \
    # kubectl
    curl -sLo /usr/local/bin/kubectl -L https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl && \
    # helm
    curl -sLO ${HELM_LOCATION}/${HELM_FILENAME} && \
    tar zxvf ${HELM_FILENAME} && mv ./linux-amd64/helm /usr/local/bin/ && \
    rm ${HELM_FILENAME} && rm -r ./linux-amd64 && \
    # helmfile
    curl -sL https://github.com/helmfile/helmfile/releases/download/${HELMFILE_VERSION}/helmfile_${HELMFILE_VERSION#v}_linux_amd64.tar.gz | tar zxf - && \
    mv helmfile /usr/local/bin && \
    rm -f README* LICENSE && \
    # sops
    curl -sLo /usr/local/bin/sops -L https://github.com/mozilla/sops/releases/download/${SOPS_VERSION}/sops-${SOPS_VERSION}.linux.amd64 && \
    # Kustomize
    curl -sL https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F${KUSTOMIZE_VERSION}/kustomize_${KUSTOMIZE_VERSION}_linux_amd64.tar.gz | tar zxf - && \
    mv kustomize /usr/local/bin && \
    # krmfnbuiltin
    curl -sLo /usr/local/bin/krmfnbuiltin https://github.com/kaweezle/krmfnbuiltin/releases/download/${KRMFNBUILTIN_VERSION}/krmfnbuiltin_${KRMFNBUILTIN_VERSION}_linux_amd64 && \
    # Age
    curl -sL https://github.com/FiloSottile/age/releases/download/${AGE_VERSION}/age-${AGE_VERSION}-linux-amd64.tar.gz | tar zxf - && \
    mv age/age* /usr/local/bin && rm -rf age && \
    # Vals
    curl -sL https://github.com/helmfile/vals/releases/download/v${VALS_VERSION}/vals_${VALS_VERSION}_linux_amd64.tar.gz | tar zxf - && \
    mv vals /usr/local/bin && \
    # Terraform
    curl -sLo /tmp/terraform.zip https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip && \
    unzip /tmp/terraform.zip -d /usr/local/bin && \
    rm /tmp/terraform.zip && \
    # Terragrunt
    curl -sLo /usr/local/bin/terragrunt https://github.com/gruntwork-io/terragrunt/releases/download/v${TERRAGRUNT_VERSION}/terragrunt_linux_amd64 && \
    chmod +x /usr/local/bin/kubectl && \
    chmod +x /usr/local/bin/helm && \
    chmod +x /usr/local/bin/helmfile && \
    chmod +x /usr/local/bin/kustomize && \
    chmod +x /usr/local/bin/krmfnbuiltin && \
    chmod +x /usr/local/bin/sops && \
    chmod +x /usr/local/bin/vals && \
    chmod +x /usr/local/bin/terragrunt

# # Azure CLI installation - commented out as not needed for now - kept for reference
# RUN curl -sL https://packages.microsoft.com/keys/microsoft.asc | gpg --dearmor > /etc/apt/trusted.gpg.d/microsoft.asc.gpg && \
#     CLI_REPO=$(lsb_release -cs) && \
#     echo "deb [arch=amd64] https://packages.microsoft.com/repos/azure-cli/ ${CLI_REPO} main" > /etc/apt/sources.list.d/azure-cli.list && \
#     apt-get update && \
#     apt-get install -y azure-cli && \
#     apt-get clean && \
#     rm -rf /var/lib/apt/lists/*


# Switch back to non-root user
USER argocd

RUN set -eux ;\
    helm plugin install https://github.com/databus23/helm-diff --version v${HELM_DIFF_VERSION} && \
    helm plugin install https://github.com/jkroepke/helm-secrets --version v${HELM_SECRETS_VERSION} && \
    helm plugin install https://github.com/mumoshu/helm-x  && \
    helm plugin install https://github.com/aslafy-z/helm-git && \
    helm plugin install https://github.com/hypnoglow/helm-s3

USER root

RUN mv /usr/local/bin/helm /usr/local/bin/helm.orig && \
    ln -sf "/home/argocd/.local/share/helm/plugins/helm-secrets/scripts/wrapper/helm.sh" /usr/local/bin/helm

USER argocd

ENV HELM_PLUGINS="/home/argocd/.local/share/helm/plugins/"

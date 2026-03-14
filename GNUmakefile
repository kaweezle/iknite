# Makefile for building Iknite packages, rootfs images, and VM images.
# cSpell: words gsub rootfull chainguard apkindex doas vhdx apks covermode coverprofile checkmake
# cSpell: words moby oras hyperv keygen nistp gomplate
SHELL := /bin/sh

# Main variables
ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
ARCH ?= $(shell uname -m)
DIST_DIR := dist
BUILD_DIR := build
KEY_NAME ?= kaweezle-devel@kaweezle.com-c9d89864.rsa
CONTAINERD_NAMESPACE ?= k8s.io
REGISTRY ?= ghcr.io
IMAGE_NAME ?= kaweezle/iknite
export CONTAINERD_NAMESPACE
CACHE_FLAG ?= "" # --no-cache
VM_STACK := openstack
export VM_STACK

########
# VERSIONS
########
# Get current git tag if we are on a tag, else empty string. Define to force a release
IKNITE_RELEASE_TAG := $(shell git describe --exact-match --tags --match="v[0-9]*" 2>/dev/null || echo "")
# If we are on a version tag, return it, else last version tag + 1 patch, with -devel suffix (e.g. v0.1.0 -> v0.1.1-devel)
IKNITE_VERSION_TAG := $(or $(IKNITE_RELEASE_TAG),$(shell git describe --abbrev=0 --tags --match="v[0-9]*" | awk -F'.' '{ printf "%s.%s.%s-devel",$$1,$$2,$$3+1;}'))
export IKNITE_VERSION_TAG
# The version is the version tag without the leading 'v'
IKNITE_VERSION := $(IKNITE_VERSION_TAG:v%=%)
export IKNITE_VERSION
# Extract Kubernetes version from go.mod file, by looking for the k8s.io/kubernetes dependency and removing the leading 'v' from the version
KUBERNETES_VERSION ?= $(shell grep 'k8s.io/kubernetes' "$(ROOT_DIR)/go.mod" | awk '{gsub(/^v/,"",$$2); print $$2;}')
export KUBERNETES_VERSION

# Get latest karmafun version from GitHub API
KARMAFUN_LATEST_VERSION := $(shell curl --silent https://api.github.com/repos/karmafun/karmafun/releases/latest | jq -r .tag_name)
KARMAFUN_VERSION := $(if $(filter null,$(KARMAFUN_LATEST_VERSION)),v0.5.0,$(KARMAFUN_LATEST_VERSION))

#######
# COMMANDS
#######
NERDCTL_CMD := $(shell command -v nerdctl 2>/dev/null)
DOCKER_CMD := $(shell command -v docker 2>/dev/null)
BUILDCTL_CMD := $(shell command -v buildctl 2>/dev/null)
BUILDX_CMD := $(shell \
	if [ -n "$(DOCKER_CMD)" ] && docker buildx version >/dev/null 2>&1; then \
		echo "docker buildx"; \
	else \
		echo ""; \
	fi)

SUDO_CMD := $(shell \
	if command -v doas >/dev/null 2>&1; then \
		echo doas; \
	elif command -v sudo >/dev/null 2>&1; then \
		echo sudo; \
	else \
		echo ""; \
	fi)

# ROOTFULL is true if the current user is root
ROOTFULL := $(shell if [ "`id -u 2>/dev/null`" == "0" ]; then echo true; fi)
# ROOT_CMD is the prefix to run commands as root
ROOT_CMD := $(if $(ROOTFULL),,$(SUDO_CMD))

# In the following we check for each command that we have found if it is actually working by running a simple command
# and checking the output.
# Each HAS_WORKING_XXX variable is set to "true" if the command is working, and empty otherwise.
HAS_WORKING_DOCKER := $(if $(DOCKER_CMD),$(if $(filter null,$(shell docker version --format json 2>/dev/null | jq -c .Server)),,true),)
HAS_WORKING_BUILDX := $(if $(BUILDX_CMD),$(shell $(BUILDX_CMD) inspect | grep -q '^Error' >/dev/null 2>&1 && echo '' || echo 'true'),)

# If we have both docker and buildx, let's create a buildx builder instance that buildctl will catch and use
ifneq ($(and $(HAS_WORKING_DOCKER),$(HAS_WORKING_BUILDX)),)
	BUILDX_BUILDER := $(shell $(BUILDX_CMD) use iknite >/dev/null 2>&1 && echo iknite || $(BUILDX_CMD) create --name iknite --use --driver docker-container --buildkitd-flags '--allow-insecure-entitlement security.insecure'  --driver-opt "image=moby/buildkit:master" --bootstrap 2>/dev/null)
	export BUILDKIT_HOST := docker-container://buildx_buildkit_$(BUILDX_BUILDER)0
endif

ifdef NERDCTL_CMD
# 'null' as output means not working
HAS_WORKING_ROOTLESS_NERDCTL := $(if $(filter null,$(shell nerdctl version --format '{{json .}}' 2>/dev/null | jq -c .Server)),,true)
HAS_WORKING_ROOTFULL_NERDCTL := $(if $(filter null,$(shell $(SUDO_CMD) nerdctl version --format '{{json .}}' 2>/dev/null | jq -c .Server)),,true)
endif

ifdef BUILDCTL_CMD
HAS_WORKING_ROOTLESS_BUILDCTL := $(if $(BUILDKIT_HOST),true,$(shell buildctl debug workers >/dev/null 2>&1 && echo 'true' || echo ''))
HAS_WORKING_ROOTFULL_BUILDCTL := $(shell $(SUDO_CMD) buildctl debug workers >/dev/null 2>&1 && echo 'true' || echo '')
endif

RUN_CONTAINER_CMD := $(if $(HAS_WORKING_DOCKER),$(DOCKER_CMD),$(if $(HAS_WORKING_ROOTLESS_NERDCTL),$(NERDCTL_CMD),$(if $(HAS_WORKING_ROOTFULL_NERDCTL),$(SUDO_CMD) $(NERDCTL_CMD),":")))
BUILD_CONTAINER_CMD := $(if $(HAS_WORKING_ROOTLESS_BUILDCTL),$(BUILDCTL_CMD),$(if $(HAS_WORKING_ROOTFULL_BUILDCTL),$(SUDO_CMD) $(BUILDCTL_CMD),":"))

SOPS_DECRYPT_CMD := sops --decrypt --input-type yaml --output-type json

#######
# END: COMMANDS
#######

# Build cache configuration for container builds. Used in GHA
BUILD_CONTAINER_CACHE_FROM_PATH := /tmp/.buildx-cache
BUILD_CONTAINER_CACHE_TO_PATH := /tmp/.buildx-cache-new
BUILD_CONTAINER_CACHE_FROM ?= type=local,src=$(BUILD_CONTAINER_CACHE_FROM_PATH)
BUILD_CONTAINER_CACHE_TO ?= type=local,dest=$(BUILD_CONTAINER_CACHE_TO_PATH)

# Variables for APK index builder image
APK_INDEX_BUILDER_IMAGE := apk-index-builder:latest
APK_INDEX_BUILDER_IMAGE_MARKER := $(DIST_DIR)/apk-index-builder.marker
APK_INDEX_BUILDER_IMAGE_DIR := $(ROOT_DIR)/.github/actions/make-apkindex
APK_INDEX_BUILDER_IMAGE_SOURCES := $(wildcard $(APK_INDEX_BUILDER_IMAGE_DIR)/*)

# Base image for rootfs, containing only the APK packages without preloaded images
IKNITE_ROOTFS_BASE := iknite-rootfs-base:$(IKNITE_VERSION_TAG)
IKNITE_ROOTFS_BASE_MARKER = $(DIST_DIR)/iknite-rootfs-base_$(IKNITE_VERSION_TAG).marker
IKNITE_ROOTFS_BASE_SOURCES := $(wildcard $(ROOT_DIR)/packaging/rootfs/base/*)

# Final rootfs image with preloaded Kubernetes images
IKNITE_ROOTFS_IMAGE := $(REGISTRY)/$(IMAGE_NAME):$(IKNITE_VERSION_TAG)-$(KUBERNETES_VERSION)
IKNITE_ROOTFS_IMAGE_MARKER = $(DIST_DIR)/iknite_$(IKNITE_VERSION_TAG)-$(KUBERNETES_VERSION).marker
IKNITE_ROOTFS_SOURCES := $(wildcard $(ROOT_DIR)/packaging/rootfs/with-images/*)

# Dev container
IKNITE_DEVCONTAINER_IMAGE_NAME := $(IMAGE_NAME)-devcontainer
IKNITE_DEVCONTAINER_IMAGE := $(REGISTRY)/$(IKNITE_DEVCONTAINER_IMAGE_NAME):$(IKNITE_VERSION_TAG)
IKNITE_DEVCONTAINER_IMAGE_MARKER := $(DIST_DIR)/iknite-devcontainer_$(IKNITE_VERSION_TAG).marker
IKNITE_DEVCONTAINER_DIR := $(ROOT_DIR)/hack/devcontainer
IKNITE_DEVCONTAINER_SOURCES := $(wildcard $(IKNITE_DEVCONTAINER_DIR)/*)

# CI Container
IKNITE_CICONTAINER_IMAGE_NAME := $(IMAGE_NAME)-cicontainer
IKNITE_CICONTAINER_IMAGE := $(REGISTRY)/$(IKNITE_CICONTAINER_IMAGE_NAME):$(IKNITE_VERSION_TAG)
IKNITE_CICONTAINER_IMAGE_MARKER := $(DIST_DIR)/iknite-cicontainer_$(IKNITE_VERSION_TAG).marker
IKNITE_CICONTAINER_DIR := $(ROOT_DIR)/hack/cicontainer
IKNITE_CICONTAINER_SOURCES := $(wildcard $(IKNITE_CICONTAINER_DIR)/*)

ifdef IKNITE_RELEASE_TAG
PUSH_IMAGES := true
IKNITE_ROOTFS_IMAGE_ADDITIONAL_TAG := $(IKNITE_ROOTFS_IMAGE):latest
IKNITE_DEVCONTAINER_IMAGE_ADDITIONAL_TAG := $(IKNITE_DEVCONTAINER_IMAGE):latest
IKNITE_CICONTAINER_IMAGE_ADDITIONAL_TAG := $(IKNITE_CICONTAINER_IMAGE):latest
# SNAPSHOT =
IKNITE_REPO_NAME := release
else
PUSH_IMAGES := false
SNAPSHOT := --snapshot
IKNITE_REPO_NAME := test
endif

ROOTFS_NAME := iknite-$(IKNITE_VERSION)-$(KUBERNETES_VERSION).rootfs.tar.gz
ROOTFS_PATH := $(DIST_DIR)/$(ROOTFS_NAME)

# Package file names
KARMAFUN_PACKAGE := karmafun-$(patsubst v%,%,$(KARMAFUN_VERSION)).$(ARCH).apk
IKNITE_PACKAGE := iknite-$(IKNITE_VERSION).$(ARCH).apk
IKNITE_IMAGES_PACKAGE := iknite-images-$(KUBERNETES_VERSION).$(ARCH).apk

# Incus Agent
INCUS_AGENT_VERSION := 6.22
INCUS_AGENT_PACKAGE := incus-agent-$(INCUS_AGENT_VERSION).$(ARCH).apk
INCUS_AGENT_SOURCE_DIR := packaging/apk/incus-agent
INCUS_AGENT_SOURCES := $(wildcard $(ROOT_DIR)/$(INCUS_AGENT_SOURCE_DIR)/*)

KUSTOMIZATION_FILES = $(wildcard $(ROOT_DIR)/packaging/apk/iknite/iknite.d/**/*)
GOLANG_FILES = $(shell find "$(ROOT_DIR)/cmd" "$(ROOT_DIR)/pkg" -type f -name '*.go' ! -name '*_test.go')
APK_FILES = $(wildcard $(ROOT_DIR)/packaging/apk/iknite/**/*)
APK_INDEX_FILE = $(DIST_DIR)/repo/$(ARCH)/APKINDEX.tar.gz

IKNITE_ROOTFS_CONTAINER_MARKER = $(DIST_DIR)/iknite-rootfs-container_$(IKNITE_VERSION_TAG)-$(KUBERNETES_VERSION).marker

# Images file names and paths
IKNITE_VM_IMAGE_QCOW2_BASENAME = iknite-vm.$(IKNITE_VERSION)-$(KUBERNETES_VERSION).qcow2
IKNITE_VM_IMAGE_VHDX_BASENAME = $(IKNITE_VM_IMAGE_QCOW2_BASENAME:.qcow2=.vhdx)
IKNITE_VM_IMAGE_QCOW2 = $(DIST_DIR)/images/$(IKNITE_VM_IMAGE_QCOW2_BASENAME)
IKNITE_VM_IMAGE_VHDX = $(DIST_DIR)/images/$(IKNITE_VM_IMAGE_VHDX_BASENAME)
IKNITE_VM_IMAGE_QCOW2_CONTAINER_MARKER = $(IKNITE_VM_IMAGE_QCOW2)-container.marker
IKNITE_VM_IMAGE_VHDX_CONTAINER_MARKER = $(IKNITE_VM_IMAGE_VHDX)-container.marker

# Incus metadata
INCUS_METADATA := $(DIST_DIR)/images/incus.tar.xz
VM_PACKAGING_DIR := $(ROOT_DIR)/packaging/vm
VM_PACKAGING_SOURCES := $(shell find $(VM_PACKAGING_DIR) -type f)
IKNITE_ROOTFS_IMAGE_INCUS_ATTACHMENT_MARKER = $(DIST_DIR)/iknite_$(IKNITE_VERSION_TAG)-$(KUBERNETES_VERSION)-incus.marker

# List of images ID in order to clean
IMAGES_ID = $(shell $(RUN_CONTAINER_CMD) image ls -q --filter "label=org.opencontainers.image.source=https://github.com/kaweezle/iknite.git" | sort -u)

# Scripts for building VM images from rootfs
BUILD_VM_IMAGE_SCRIPTS = $(ROOT_DIR)/packaging/scripts/build-vm-image.sh $(ROOT_DIR)/packaging/scripts/configure-vm-image.sh

SECRETS_FILE := $(ROOT_DIR)/deploy/k8s/argocd/secrets/secrets.sops.yaml
SSH_KNOWN_HOSTS_FILE := $(ROOT_DIR)/hack/devcontainer/iknite_known_hosts

.PHONY: help
help: # ignore checkmake
	@echo "Iknite build targets"
	@echo ""
	@echo "Step targets:"
	@echo "  make info                           Show build configuration information"
	@echo "  make extract-key                    Extract signing key from sops file"
	@echo "  make goreleaser                     Build iknite package (goreleaser)"
	@echo "  make fetch-karmafun                 Download latest karmafun APK into dist/"
	@echo "  make images-apk                     Build iknite-images APK"
	@echo "  make incus-agent-apk                Build incus-agent APK"
	@echo "  make apk-repo                       Create APK repository in dist/repo"
	@echo "  make upload-apk-repo                Upload APK repository with terragrunt"
	@echo "  make rootfs-base-image              Build rootfs base image"
	@echo "  make rootfs-container               Add preloaded images into rootfs container"
	@echo "  make rootfs                         Build rootfs"
	@echo "  make rootfs-image                   Build final rootfs image"
	@echo "  make vm-image                       Build VM images (qcow2, vhdx)"
	@echo "  make incus-metadata                 Build Incus metadata tarball (dist/images/incus.tar.xz)"
	@echo "  make rootfs-image-incus-attachment  Attach Incus metadata to rootfs image in container registry with oras"
	@echo "  make vm-container-images            Build VM images as container images"
	@echo "  make clean                          Remove build artifacts and temp container"
	@echo "  make all                            Run full pipeline"
	@echo "  make ssh-key                        Extract SSH key for iknite VMs from sops file"
	@echo "  make vm-known-hosts                 Extract VM SSH host public key to ~/.ssh/iknite_known_hosts"
	@echo "  make generate-vm-host-keys          Generate new fixed SSH host keys for iknite VMs"
	@echo "  make vm-ssh                         Connect to the E2E test VM using the fixed host key"
	@echo "  make e2e-tg-init                    Initialize terragrunt E2E test configuration"
	@echo "  make e2e-tg-refresh                 Refresh terragrunt E2E test state without applying changes"
	@echo "  make e2e-tg-apply                   Apply terragrunt E2E test configuration to create E2E test VM"
	@echo "  make e2e-tg-destroy                 Destroy E2E test VM with terragrunt"
	@echo "  make e2e-check-argocd               Check ArgoCD application status for E2E test cluster"
	@echo "  make release-files                  Generate SHA256SUMS file for release artifacts"
	@echo "  make devcontainer                   Build dev container image"
	@echo ""
	@echo "File targets (examples):"
	@echo "  make dist/iknite-<version>.$(ARCH).apk"
	@echo "  make dist/iknite-images-<k8s-version>.$(ARCH).apk"
	@echo "  make dist/karmafun-<version>.$(ARCH).apk"
	@echo "  make dist/SHA256SUMS"
	@echo ""
	@echo "Common variables (override with VAR=value):"
	@echo "  ARCH=$(ARCH)"
	@echo "  KUBERNETES_VERSION=$(KUBERNETES_VERSION)"
	@echo "  IKNITE_RELEASE_TAG=$(IKNITE_RELEASE_TAG)"
	@echo "  IKNITE_VERSION=$(IKNITE_VERSION)"
	@echo "  IKNITE_REPO_NAME=$(IKNITE_REPO_NAME)"
	@echo "  CACHE_FLAG=$(CACHE_FLAG)"
	@echo "  VM_STACK=$(VM_STACK)"
	@echo "  SNAPSHOT=$(SNAPSHOT)"
	@echo "  PUSH_IMAGES=$(PUSH_IMAGES)"

.PHONY: info
info:
	@echo "Is Release:            $(if $(IKNITE_RELEASE_TAG),yes,no)"
	@echo "Iknite version:        $(IKNITE_VERSION)"
	@echo "Kubernetes version:    $(KUBERNETES_VERSION)"
	@echo "Architecture:          $(ARCH)"
	@echo "Container runner cmd:  $(RUN_CONTAINER_CMD)"
	@echo "Container builder cmd: $(BUILD_CONTAINER_CMD)"
	@echo "Using build cache:     $(if $(filter --no-cache,$(CACHE_FLAG)),no,yes)"
	@echo "Pushing images:        $(PUSH_IMAGES)"
	@echo "Root:                  $(if $(ROOTFULL),yes,no)"

.PHONY: all
all: extract-key goreleaser fetch-karmafun images-apk incus-agent-apk apk-repo rootfs-base-image rootfs-container rootfs rootfs-image vm-image incus-metadata vm-container-images rootfs-image-incus-attachment

print-% : ; $(info $* is a $(flavor $*) variable set to [$($*)]) @true

# Check prerequisites before running any target and install tools with aqua if needed
.PHONY: check-prerequisites
check-prerequisites:
	@mkdir -p "$(BUILD_DIR)" "$(DIST_DIR)"
	@command -v jq >/dev/null 2>&1 || { echo "Error: jq is not installed"; exit 1; }
	@command -v curl >/dev/null 2>&1 || { echo "Error: curl is not installed"; exit 1; }
	@command -v bsdtar >/dev/null 2>&1 || { echo "Error: bsdtar is not installed"; exit 1; }
	@command -v aqua >/dev/null 2>&1 || { echo "Error: aqua is not installed"; exit 1; }
	@command -v yq >/dev/null 2>&1 || { echo "Error: yq is not installed"; exit 1; }
	@aqua i
	@test "$(RUN_CONTAINER_CMD) != :" || { echo "Error: No container runner (either docker or nerdctl) available"; exit 1; }
	@test "$(BUILD_CONTAINER_CMD) != :" || { echo "Error: No container image builder (either buildctl or docker buildx) available"; exit 1; }
	@mkdir -p "$(BUILD_CONTAINER_CACHE_FROM_PATH)" "$(BUILD_CONTAINER_CACHE_TO_PATH)"

# Login to container registry using credentials from sops file or GitHub Actions secrets
.PHONY: container-login
container-login: $(SECRETS_FILE) | check-prerequisites
	@echo "Logging into container registry $(REGISTRY)..."
	if [ -n "$(GITHUB_TOKEN)" ] && [ -n "$(GITHUB_ACTOR)" ]; then \
		echo "$$GITHUB_TOKEN" | $(RUN_CONTAINER_CMD) login $(REGISTRY) --username $(shell echo "$$GITHUB_ACTOR" | sed 's/[^a-zA-Z0-9]/-/g') --password-stdin; \
	else \
		USERNAME=`$(SOPS_DECRYPT_CMD) $< | jq -r '.data.docker.registry.username'` ; \
		echo "login as $$USERNAME to $(REGISTRY)..."; \
		$(SOPS_DECRYPT_CMD) $< | jq -r '.data.docker.registry.password' |  $(RUN_CONTAINER_CMD) login $(REGISTRY) --username "$$USERNAME" --password-stdin; \
	fi

# Extract signing key from sops file
$(ROOT_DIR)/$(KEY_NAME): $(SECRETS_FILE) | check-prerequisites
	@echo "Extracting signing key '$@' from $<..."
	$(SOPS_DECRYPT_CMD) $< | jq -r '.data.apk_signing_key.private_key' > "$@"
	chmod 600 "$@"

.PHONY: extract-key
extract-key: $(ROOT_DIR)/$(KEY_NAME)

# Goreleaser build
$(DIST_DIR)/$(IKNITE_PACKAGE) $(DIST_DIR)/metadata.json $(DIST_DIR)/iknite_linux_amd64_v1/iknite &: $(GOLANG_FILES) $(APK_FILES) go.mod .goreleaser.yaml | check-prerequisites
	goreleaser release --skip=publish $(SNAPSHOT) --clean

.PHONY: goreleaser
goreleaser: $(DIST_DIR)/$(IKNITE_PACKAGE) $(DIST_DIR)/metadata.json $(DIST_DIR)/iknite_linux_amd64_v1/iknite

# Download latest karmafun release from GitHub
$(DIST_DIR)/$(KARMAFUN_PACKAGE): | check-prerequisites
	@echo "Latest karmafun version is $(KARMAFUN_VERSION)"
	curl -o $@ -L "https://github.com/karmafun/karmafun/releases/download/$(KARMAFUN_VERSION)/$(KARMAFUN_PACKAGE)"

.PHONY: fetch-karmafun
fetch-karmafun: $(DIST_DIR)/$(KARMAFUN_PACKAGE)

# Build iknite-images APK by extracting image list from kustomization and using melange in a container
$(DIST_DIR)/$(IKNITE_IMAGES_PACKAGE): $(DIST_DIR)/iknite_linux_amd64_v1/iknite $(KUSTOMIZATION_FILES) $(ROOT_DIR)/$(KEY_NAME)| check-prerequisites
	BUILD_DIR_SUFFIX="build/apk/iknite-images"; \
	BUILD_DIR="$(ROOT_DIR)/$$BUILD_DIR_SUFFIX"; \
	rm -rf "$$BUILD_DIR"; \
	mkdir -p "$$BUILD_DIR"; \
	echo "KUBERNETES_VERSION: $(KUBERNETES_VERSION)" > "$$BUILD_DIR/.env"; \
	"$(DIST_DIR)/iknite_linux_amd64_v1/iknite" kustomize -d "$(ROOT_DIR)/packaging/apk/iknite/iknite.d" print | grep image: | awk '{ print $$2; }' > "$$BUILD_DIR/image-list.txt"; \
	"$(DIST_DIR)/iknite_linux_amd64_v1/iknite" info images >> "$$BUILD_DIR/image-list.txt"; \
	sort -u "$$BUILD_DIR/image-list.txt" -o "$$BUILD_DIR/image-list.txt"; \
	$(RUN_CONTAINER_CMD) run --privileged --rm -v "$(ROOT_DIR):/work" cgr.dev/chainguard/melange \
		build packaging/apk/iknite-images/iknite-images.yaml \
		--arch "$(ARCH)" \
		--vars-file "$$BUILD_DIR_SUFFIX/.env" \
		--source-dir "$$BUILD_DIR_SUFFIX" \
		--out-dir "dist" \
		--signing-key $(KEY_NAME) \
		--generate-index=false; \
	$(ROOT_CMD) chown -R "$$(id -u):$$(id -g)" "$(DIST_DIR)"; \
	(cd "$(DIST_DIR)/$(ARCH)" && for f in *.apk; do mv "$$f" "../$$(echo "$$f" | sed 's/\-r0.apk$$//').$(ARCH).apk"; done); \
	rmdir "$(DIST_DIR)/$(ARCH)/"

.PHONY: images-apk
images-apk: $(DIST_DIR)/$(IKNITE_IMAGES_PACKAGE)

# Build incus-agent APK using melange in a container
$(DIST_DIR)/$(INCUS_AGENT_PACKAGE): $(INCUS_AGENT_SOURCES) $(ROOT_DIR)/$(KEY_NAME) | check-prerequisites
	export INCUS_AGENT_VERSION="$(INCUS_AGENT_VERSION)"; \
	$(RUN_CONTAINER_CMD) run --privileged --rm -v "$(ROOT_DIR):/work" cgr.dev/chainguard/melange \
		build packaging/apk/incus-agent/incus-agent.yaml \
		--arch "$(ARCH)" \
		--source-dir "$(INCUS_AGENT_SOURCE_DIR)" \
		--out-dir "dist" \
		--signing-key $(KEY_NAME) \
		--generate-index=false; \
	$(ROOT_CMD) chown -R "$$(id -u):$$(id -g)" "$(DIST_DIR)"; \
	(cd "$(DIST_DIR)/$(ARCH)" && for f in *.apk; do mv "$$f" "../$$(echo "$$f" | sed 's/\-r0.apk$$//').$(ARCH).apk"; done); \
	rmdir "$(DIST_DIR)/$(ARCH)/"

.PHONY: incus-agent-apk
incus-agent-apk: $(DIST_DIR)/$(INCUS_AGENT_PACKAGE)

$(APK_INDEX_BUILDER_IMAGE_MARKER): $(APK_INDEX_BUILDER_IMAGE_SOURCES) | check-prerequisites
	$(BUILD_CONTAINER_CMD) build \
		--frontend dockerfile.v0 \
		--import-cache=$(BUILD_CONTAINER_CACHE_FROM) \
		--export-cache=$(BUILD_CONTAINER_CACHE_TO) \
		--local "context=$(APK_INDEX_BUILDER_IMAGE_DIR)" \
		--local "dockerfile=$(APK_INDEX_BUILDER_IMAGE_DIR)" \
		$(CACHE_FLAG) \
		--output "type=docker,name=$(APK_INDEX_BUILDER_IMAGE),push=false" | $(RUN_CONTAINER_CMD) load
	touch "$@"

# Create APK repository in dist/repo by generating APKINDEX.tar.gz with the built packages and copying APKs
$(APK_INDEX_FILE): $(DIST_DIR)/$(KARMAFUN_PACKAGE) $(DIST_DIR)/$(IKNITE_PACKAGE) $(DIST_DIR)/$(IKNITE_IMAGES_PACKAGE) $(DIST_DIR)/$(INCUS_AGENT_PACKAGE) $(ROOT_DIR)/$(KEY_NAME) $(APK_INDEX_BUILDER_IMAGE_MARKER) | check-prerequisites
	rm -rf "$(DIST_DIR)/repo"
	mkdir -p "$(DIST_DIR)/repo"
	$(RUN_CONTAINER_CMD) run --rm \
		-v "$(ROOT_DIR):/workspace" \
		-e "INPUT_APK_FILES=$(DIST_DIR)/*.apk" \
		-e "INPUT_DESTINATION=$(DIST_DIR)/repo" \
		-e "INPUT_SIGNATURE_KEY_NAME=$(KEY_NAME)" \
		-e "INPUT_SIGNATURE_KEY=$$(cat "$(ROOT_DIR)/$(KEY_NAME)")" \
		-e "GITHUB_WORKSPACE=/workspace" \
			$(APK_INDEX_BUILDER_IMAGE); \
	$(ROOT_CMD) chown -R "$$(id -u):$$(id -g)" "$(DIST_DIR)/repo"

.PHONY: apk-repo
apk-repo: $(APK_INDEX_FILE)

# Upload APK repository with terragrunt by applying Terraform configuration in deploy/iac/iknite/$(IKNITE_REPO_NAME)repo
.PHONY: upload-apk-repo
upload-apk-repo: $(APK_INDEX_FILE) | check-prerequisites
	cd "$(ROOT_DIR)/deploy/iac/iknite/$(IKNITE_REPO_NAME)repo" && terragrunt init && terragrunt apply -auto-approve

# Build rootfs base image
$(IKNITE_ROOTFS_BASE_MARKER): $(DIST_DIR)/$(KARMAFUN_PACKAGE) $(DIST_DIR)/$(IKNITE_PACKAGE) $(DIST_DIR)/$(INCUS_AGENT_PACKAGE) $(IKNITE_ROOTFS_BASE_SOURCES) | check-prerequisites
	BUILD_DIR_PATH="$(BUILD_DIR)/rootfs/base"; \
	rm -rf "$$BUILD_DIR_PATH"; \
	mkdir -p "$$BUILD_DIR_PATH"; \
	cp -r "$(ROOT_DIR)/packaging/rootfs/base/." "$$BUILD_DIR_PATH/"; \
	cp "$(DIST_DIR)/$(IKNITE_PACKAGE)" "$$BUILD_DIR_PATH/"; \
	cp "$(DIST_DIR)/$(KARMAFUN_PACKAGE)" "$$BUILD_DIR_PATH/"; \
	cp "$(DIST_DIR)/$(INCUS_AGENT_PACKAGE)" "$$BUILD_DIR_PATH/"; \
	$(BUILD_CONTAINER_CMD) build \
		--frontend dockerfile.v0 \
		--import-cache=$(BUILD_CONTAINER_CACHE_FROM) \
		--export-cache=$(BUILD_CONTAINER_CACHE_TO) \
		--local "context=$$BUILD_DIR_PATH" \
		--local "dockerfile=$$BUILD_DIR_PATH" \
		--opt "build-arg:IKNITE_REPO_URL=https://static.iknite.app/$(IKNITE_REPO_NAME)/" \
		--opt "build-arg:IKNITE_VERSION=$(IKNITE_VERSION)" \
		$(CACHE_FLAG) \
		--output "type=docker,dest=-,name=$(IKNITE_ROOTFS_BASE),push=false" | $(RUN_CONTAINER_CMD) load
	@touch "$@"

.PHONY: rootfs-base-image
rootfs-base-image: $(IKNITE_ROOTFS_BASE_MARKER)

# Create a container from the base image and install the iknite-images package to preload the Kubernetes images
$(IKNITE_ROOTFS_CONTAINER_MARKER): $(DIST_DIR)/$(IKNITE_IMAGES_PACKAGE) $(IKNITE_ROOTFS_BASE_MARKER) | check-prerequisites
	$(RUN_CONTAINER_CMD) rm -f iknite-rootfs 2>/dev/null || true
	$(RUN_CONTAINER_CMD) run \
		--name iknite-rootfs \
		--device /dev/fuse --cap-add SYS_ADMIN \
		-v "$(realpath $(DIST_DIR)):/apks" \
		-e "IKNITE_IMAGES_PACKAGE=$(IKNITE_IMAGES_PACKAGE)" \
		"$(IKNITE_ROOTFS_BASE)" \
		/bin/sh -c 'apk --no-cache add /apks/$$IKNITE_IMAGES_PACKAGE; apk del iknite-images'
	@touch "$@"

.PHONY: rootfs-container
rootfs-container: $(IKNITE_ROOTFS_CONTAINER_MARKER)

# Export the container filesystem as a tar.gz archive, excluding the .dockerenv file and dev/console device
$(ROOTFS_PATH): $(IKNITE_ROOTFS_CONTAINER_MARKER) | check-prerequisites
	rm -f "$@" || true
	$(RUN_CONTAINER_CMD) export iknite-rootfs | bsdtar -zcf "$@" --exclude=.dockerenv --exclude=dev/console @-

.PHONY: rootfs
rootfs: $(ROOTFS_PATH)

# Create final rootfs flat image from the rootfs tarball
$(IKNITE_ROOTFS_IMAGE_MARKER): $(ROOTFS_PATH) $(IKNITE_ROOTFS_SOURCES) | check-prerequisites
	BUILD_DIR_PATH="$(BUILD_DIR)/rootfs/with-images"; \
	rm -rf "$$BUILD_DIR_PATH"; \
	mkdir -p "$$BUILD_DIR_PATH"; \
	cp -r "$(ROOT_DIR)/packaging/rootfs/with-images/." "$$BUILD_DIR_PATH/"; \
	cp "$(DIST_DIR)/$(ROOTFS_NAME)" "$$BUILD_DIR_PATH/$(ROOTFS_NAME)"; \
	$(BUILD_CONTAINER_CMD) build \
		--frontend dockerfile.v0 \
		--import-cache=$(BUILD_CONTAINER_CACHE_FROM) \
		--export-cache=$(BUILD_CONTAINER_CACHE_TO) \
		--local "context=$$BUILD_DIR_PATH" \
		--local "dockerfile=$$BUILD_DIR_PATH" \
		--opt "build-arg:IKNITE_VERSION=$(IKNITE_VERSION)" \
		--opt "build-arg:KUBERNETES_VERSION=$(KUBERNETES_VERSION)" \
		$(CACHE_FLAG) \
		--output "type=docker,name=$(IKNITE_ROOTFS_IMAGE)" | $(RUN_CONTAINER_CMD) load; \
	if [ "$(PUSH_IMAGES)" = "true" ]; then \
		$(RUN_CONTAINER_CMD) push "$(IKNITE_ROOTFS_IMAGE)"; \
		if [ -n "$(IKNITE_ROOTFS_IMAGE_ADDITIONAL_TAG)" ]; then \
			$(RUN_CONTAINER_CMD) tag "$(IKNITE_ROOTFS_IMAGE)" "$(IKNITE_ROOTFS_IMAGE_ADDITIONAL_TAG)"; \
			$(RUN_CONTAINER_CMD) push "$(IKNITE_ROOTFS_IMAGE_ADDITIONAL_TAG)"; \
		fi; \
	fi
	@touch "$@"

.PHONY: rootfs-image
rootfs-image: $(IKNITE_ROOTFS_IMAGE_MARKER)

# Create the VM images from the rootfs tarball
$(IKNITE_VM_IMAGE_QCOW2) $(IKNITE_VM_IMAGE_VHDX) &: $(ROOTFS_PATH) $(BUILD_VM_IMAGE_SCRIPTS) | check-prerequisites
	$(ROOT_CMD) rm -rf "$(ROOT_DIR)/dist/images" || true
	mkdir -p "$(ROOT_DIR)/dist/images"
	$(ROOT_CMD) "$(ROOT_DIR)/packaging/scripts/build-vm-image.sh"
	$(ROOT_CMD) chown -R "$$(id -u):$$(id -g)" "$(ROOT_DIR)/dist/images" || true

.PHONY: vm-image
vm-image: $(IKNITE_VM_IMAGE_QCOW2) $(IKNITE_VM_IMAGE_VHDX)

# Build Incus metadata tarball (dist/images/incus.tar.xz) from gomplate template
$(INCUS_METADATA): $(VM_PACKAGING_SOURCES) $(IKNITE_VM_IMAGE_QCOW2) | check-prerequisites
	BUILD_DIR_PATH="$(BUILD_DIR)/incus-metadata"; \
	rm -rf "$$BUILD_DIR_PATH"; \
	mkdir -p "$$BUILD_DIR_PATH"; \
	KUBERNETES_VERSION="$(KUBERNETES_VERSION)" \
	IKNITE_VERSION="$(IKNITE_VERSION)" \
	IKNITE_VERSION_TAG="$(IKNITE_VERSION_TAG)" \
	IMAGE=$(IKNITE_VM_IMAGE_QCOW2) \
		gomplate -f "$(VM_PACKAGING_DIR)/metadata.yaml.tmpl" -o "$${BUILD_DIR_PATH}/metadata.yaml"; \
	cp -r "$(VM_PACKAGING_DIR)/templates" "$${BUILD_DIR_PATH}/templates"; \
	cd "$${BUILD_DIR_PATH}" && bsdtar -cJf "$(shell realpath "$@")" *

.PHONY: incus-metadata
incus-metadata: $(INCUS_METADATA)

# Attach the Incus metadata tarball to the rootfs image in the container registry using oras, so it can be consumed by Incus when pulling the image
$(IKNITE_ROOTFS_IMAGE_INCUS_ATTACHMENT_MARKER): $(IKNITE_ROOTFS_IMAGE_MARKER) $(INCUS_METADATA) | check-prerequisites container-login
	cd "$(ROOT_DIR)/dist/images"; \
	oras attach "$(IKNITE_ROOTFS_IMAGE)" \
	--artifact-type application/vnd.incus.metadata \
	incus.tar.xz:application/x-xz
	touch "$@"

.PHONY: rootfs-image-incus-attachment
rootfs-image-incus-attachment: $(IKNITE_ROOTFS_IMAGE_INCUS_ATTACHMENT_MARKER)

# Push the VM images to the container registry using oras, with appropriate annotations and tags
$(IKNITE_VM_IMAGE_VHDX_CONTAINER_MARKER): $(IKNITE_VM_IMAGE_VHDX) | check-prerequisites container-login
	cd "$(ROOT_DIR)/dist/images"; \
	IMAGE_TAG="$(REGISTRY)/$(IMAGE_NAME)-vm-vhdx:$(IKNITE_VERSION_TAG)-$(KUBERNETES_VERSION)"; \
	oras push $$IMAGE_TAG \
	--artifact-type application/vnd.oci.image.layer.vhdx \
	--annotation org.opencontainers.image.title="iknite-vm-$(IKNITE_VERSION_TAG)-$(KUBERNETES_VERSION).vhdx" \
	--annotation org.opencontainers.image.version="$(IKNITE_VERSION_TAG)" \
	--annotation org.opencontainers.image.description="VM image for iknite $(IKNITE_VERSION) with Kubernetes $(KUBERNETES_VERSION)" \
	$(IKNITE_VM_IMAGE_VHDX_BASENAME):application/x-hyperv-disk; \
	if [ "$(IKNITE_REPO_NAME)" = "release" ]; then \
		oras tag "$$IMAGE_TAG" "$(REGISTRY)/$(IMAGE_NAME)-vm-vhdx:latest"; \
	fi
	touch "$@"

$(IKNITE_VM_IMAGE_QCOW2_CONTAINER_MARKER): $(IKNITE_VM_IMAGE_QCOW2) $(INCUS_METADATA) | check-prerequisites container-login
	cd "$(ROOT_DIR)/dist/images"; \
	IMAGE_TAG="$(REGISTRY)/$(IMAGE_NAME)-vm-qcow2:$(IKNITE_VERSION_TAG)-$(KUBERNETES_VERSION)"; \
	oras push $$IMAGE_TAG \
	--artifact-type application/vnd.oci.image.layer.qcow2 \
	--annotation org.opencontainers.image.title="iknite-vm-$(IKNITE_VERSION_TAG)-$(KUBERNETES_VERSION).qcow2" \
	--annotation org.opencontainers.image.version="$(IKNITE_VERSION_TAG)" \
	--annotation org.opencontainers.image.description="VM image for iknite $(IKNITE_VERSION) with Kubernetes $(KUBERNETES_VERSION)" \
	$(IKNITE_VM_IMAGE_QCOW2_BASENAME):application/x-qcow2 \
	incus.tar.xz:application/vnd.incus.metadata; \
	if [ "$(IKNITE_REPO_NAME)" = "release" ]; then \
		oras tag "$$IMAGE_TAG" "$(REGISTRY)/$(IMAGE_NAME)-vm-qcow2:latest"; \
	fi
	touch "$@"

.PHONY: vm-container-images
vm-container-images: $(IKNITE_VM_IMAGE_QCOW2_CONTAINER_MARKER) $(IKNITE_VM_IMAGE_VHDX_CONTAINER_MARKER)

.PHONY: publish-vm-images
publish-vm-images: $(IKNITE_VM_IMAGE_QCOW2_CONTAINER_MARKER) $(IKNITE_VM_IMAGE_VHDX_CONTAINER_MARKER) | check-prerequisites
	@echo "VM images have been pushed to the container registry. If this is a release build, they are also tagged as latest."
	cd $(ROOT_DIR)/deploy/iac/iknite/iknite-public-images; \
	terragrunt run --graph --non-interactive apply -- -auto-approve

.PHONY: e2e-tg-init
e2e-tg-init: $(IKNITE_VM_IMAGE_QCOW2) $(INCUS_METADATA)
	cd "$(ROOT_DIR)/deploy/iac/iknite/$(VM_STACK)/iknite-image"; \
	terragrunt run --graph init

.PHONY: e2e-tg-refresh
e2e-tg-refresh:
	cd "$(ROOT_DIR)/deploy/iac/iknite/$(VM_STACK)/iknite-image"; \
	terragrunt run --graph apply --non-interactive -- -auto-approve -refresh-only

.PHONY: e2e-tg-apply
e2e-tg-apply:
	cd "$(ROOT_DIR)/deploy/iac/iknite/$(VM_STACK)/iknite-image"; \
	terragrunt run --graph apply --non-interactive -- -auto-approve

.PHONY: e2e-tg-apply-vm
e2e-tg-apply-vm:
	cd "$(ROOT_DIR)/deploy/iac/iknite/$(VM_STACK)/iknite-vm"; \
	terragrunt run --graph apply --non-interactive -- -auto-approve

.PHONY: e2e-tg-destroy
e2e-tg-destroy:
	cd "$(ROOT_DIR)/deploy/iac/iknite/$(VM_STACK)/iknite-vm"; \
	terragrunt run --graph destroy --non-interactive -- -auto-approve

.PHONY: e2e-check-argocd
e2e-check-argocd:
	@echo "Checking ArgoCD application status..."
	./test/e2e/argocd-checker.sh

.PHONY: e2e
e2e: e2e-tg-init e2e-tg-apply e2e-check-argocd e2e-tg-destroy


RELEASE_FILES := \
	$(DIST_DIR)/$(IKNITE_PACKAGE) \
	$(DIST_DIR)/$(IKNITE_IMAGES_PACKAGE) \
	$(ROOT_DIR)/packaging/rootfs/base/$(KEY_NAME).pub \
	$(ROOT_DIR)/Get-Iknite.ps1 \
	$(ROOT_DIR)/Get-IkniteVM.ps1 \
	$(ROOT_DIR)/get-iknite.sh \
	$(INCUS_METADATA) \
	$(ROOTFS_PATH)

RELEASE_DIR := $(ROOT_DIR)/release

# Link release files into release/ directory for easier access when creating GitHub releases or similar
RELEASE_LINKS := $(foreach f,$(RELEASE_FILES),$(RELEASE_DIR)/$(notdir $(f)))

$(RELEASE_DIR):
	mkdir -p "$@"

define RELEASE_LINK_RULE
$(RELEASE_DIR)/$(notdir $(1)): $(1) | $(RELEASE_DIR)
	@echo "Linking `realpath "$(1)"` to $$@..."
	ln -sf `realpath "$(1)"` "$$@"
endef
$(foreach f,$(RELEASE_FILES),$(eval $(call RELEASE_LINK_RULE,$(f))))

$(RELEASE_DIR)/SHA256SUMS: $(RELEASE_LINKS) | $(RELEASE_DIR)
	@echo "Generating SHA256SUMS file for release artifacts..."
	cd "$(RELEASE_DIR)" && sha256sum $(notdir $(RELEASE_FILES)) > SHA256SUMS

release-files: $(RELEASE_DIR)/SHA256SUMS

.PHONY: clean
clean:
	rm -rf "$(BUILD_DIR)"
	rm -rf "$(DIST_DIR)"
	$(RUN_CONTAINER_CMD) rm -f iknite-rootfs >/dev/null 2>&1 || true
	$(BUILD_CONTAINER_CMD) prune >/dev/null 2>&1 || true
	$(RUN_CONTAINER_CMD) image rm -f $(IMAGES_ID) >/dev/null 2>&1 || true

.PHONY: test
test: check-prerequisites
	@echo "Running tests..."
	go test -v -race -covermode=atomic -coverprofile=coverage.out ./...

$(HOME)/.ssh/iknite: $(SECRETS_FILE) | check-prerequisites
	@echo "Extracting SSH key for iknite from $<..."
	mkdir -p "$(HOME)/.ssh"
	chmod 700 "$(HOME)/.ssh"
	$(SOPS_DECRYPT_CMD) $< | jq -r '.data.iknite_vm.ssh_private_key' > "$@"
	chmod 600 "$@"

.PHONY: ssh-key
ssh-key: $(HOME)/.ssh/iknite

$(HOME)/.ssh/iknite_known_hosts: $(SECRETS_FILE) | check-prerequisites
	@echo "Extracting VM SSH host public key for known_hosts from $<..."
	mkdir -p "$(HOME)/.ssh"
	chmod 700 "$(HOME)/.ssh"
	$(SOPS_DECRYPT_CMD) $< | jq -r '"* " + .data.iknite_vm.ssh_host_ecdsa_public' > "$@"
	chmod 600 "$@"

.PHONY: vm-known-hosts
vm-known-hosts: $(HOME)/.ssh/iknite_known_hosts ## Extract VM SSH host public key to ~/.ssh/iknite_known_hosts

.PHONY: generate-vm-host-keys
generate-vm-host-keys: ## Generate new fixed SSH host keys for iknite VMs and update devcontainer known_hosts
	@echo "Generating new SSH host key pair for iknite VMs..."
	@TMP_KEY=$$(mktemp) && \
	echo "y" | ssh-keygen -t ecdsa-sha2-nistp256 -b 256 -C "iknite-vm-host-key" -f "$$TMP_KEY" -N "" -q && \
	PRIVATE_KEY=$$(cat "$$TMP_KEY" | jq -Rs 'split("\n") | join("\n")') && \
	PUBLIC_KEY=$$(cat "$$TMP_KEY.pub") && \
	rm -f "$$TMP_KEY" "$$TMP_KEY.pub" && \
	echo "# cSpell: disable" > "$(SSH_KNOWN_HOSTS_FILE)" && \
	echo "# Pre-trusted SSH host key for iknite VMs." >> "$(SSH_KNOWN_HOSTS_FILE)" && \
	echo "# The iknite VM always presents this host key (configured via cloud-init ssh_keys)," >> "$(SSH_KNOWN_HOSTS_FILE)" && \
	echo "# allowing strict host key verification without accepting unknown keys." >> "$(SSH_KNOWN_HOSTS_FILE)" && \
	echo "# Use: ssh -o Hostname=<VM_IP> iknite-vm" >> "$(SSH_KNOWN_HOSTS_FILE)" && \
	echo "* $$PUBLIC_KEY" >> "$(SSH_KNOWN_HOSTS_FILE)" && \
	PUBLIC_KEY_ESCAPED=$$(echo "$$PUBLIC_KEY" | jq -Rs 'split("\n") | join("\n")') && \
	sops set $(SECRETS_FILE) '["data"]["iknite_vm"]["ssh_host_ecdsa_private"]' "$$PRIVATE_KEY" && \
	sops set $(SECRETS_FILE) '["data"]["iknite_vm"]["ssh_host_ecdsa_public"]' "$$PUBLIC_KEY_ESCAPED" && \
	echo "" && \
	echo "Generated new SSH host key pair." && \
	echo "Public key: $$PUBLIC_KEY" && \
	echo "" && \
	echo "The following files have been modified and need to be committed:" && \
	echo "  - $(SSH_KNOWN_HOSTS_FILE)" && \
	echo "  - $(SECRETS_FILE)"

.PHONY: vm-ssh
vm-ssh: $(HOME)/.ssh/iknite $(HOME)/.ssh/iknite_known_hosts ## Connect to the E2E test VM using the fixed host key
	@VM_IP=$$(cd "$(ROOT_DIR)/deploy/iac/iknite/$(VM_STACK)/iknite-vm" && terragrunt output --raw --non-interactive instances 2>/dev/null | jq -r '."iknite-vm-instance".access_ip_v4' 2>/dev/null || echo ""); \
	if [ -z "$$VM_IP" ]; then \
		echo "Error: Could not determine VM IP. Run 'make e2e-tg-apply' first."; \
		exit 1; \
	fi; \
	echo "Connecting to iknite VM at $$VM_IP..."; \
	ssh -o Hostname="$$VM_IP" iknite-vm

.PHONY: rotate-cache
rotate-cache:
	@echo "Rotating build container cache..."
	if [ -d "$(BUILD_CONTAINER_CACHE_FROM_PATH)" ]; then \
		rm -rf "$(BUILD_CONTAINER_CACHE_FROM_PATH)"; \
	fi; \
	if [ -d "$(BUILD_CONTAINER_CACHE_TO_PATH)" ]; then \
		mv "$(BUILD_CONTAINER_CACHE_TO_PATH)" "$(BUILD_CONTAINER_CACHE_FROM_PATH)"; \
	fi

$(IKNITE_DEVCONTAINER_IMAGE_MARKER): $(IKNITE_DEVCONTAINER_SOURCES) | check-prerequisites container-login
	$(BUILD_CONTAINER_CMD) build \
		--frontend dockerfile.v0 \
		--import-cache=$(BUILD_CONTAINER_CACHE_FROM) \
		--export-cache=$(BUILD_CONTAINER_CACHE_TO) \
		--local "context=$(IKNITE_DEVCONTAINER_DIR)" \
		--local "dockerfile=$(IKNITE_DEVCONTAINER_DIR)" \
		--opt "build-arg:IKNITE_REPO_URL=https://static.iknite.app/$(IKNITE_REPO_NAME)/" \
		--opt "build-arg:IKNITE_VERSION=$(IKNITE_VERSION)" \
		$(CACHE_FLAG) \
		--output "type=docker,dest=-,name=$(IKNITE_DEVCONTAINER_IMAGE),push=false" | $(RUN_CONTAINER_CMD) load
	if [ "$(PUSH_IMAGES)" = "true" ]; then \
		$(RUN_CONTAINER_CMD) push "$(IKNITE_DEVCONTAINER_IMAGE)"; \
		if [ -n "$(IKNITE_DEVCONTAINER_IMAGE_ADDITIONAL_TAG)" ]; then \
			$(RUN_CONTAINER_CMD) tag "$(IKNITE_DEVCONTAINER_IMAGE)" "$(IKNITE_DEVCONTAINER_IMAGE_ADDITIONAL_TAG)"; \
			$(RUN_CONTAINER_CMD) push "$(IKNITE_DEVCONTAINER_IMAGE_ADDITIONAL_TAG)"; \
		fi; \
	fi
	@touch "$@"

.PHONY: devcontainer
devcontainer: $(IKNITE_DEVCONTAINER_IMAGE_MARKER)

$(IKNITE_CICONTAINER_IMAGE_MARKER): $(IKNITE_CICONTAINER_SOURCES) | check-prerequisites container-login
	$(BUILD_CONTAINER_CMD) build \
		--frontend dockerfile.v0 \
		--import-cache=$(BUILD_CONTAINER_CACHE_FROM) \
		--export-cache=$(BUILD_CONTAINER_CACHE_TO) \
		--local "context=$(IKNITE_CICONTAINER_DIR)" \
		--local "dockerfile=$(IKNITE_CICONTAINER_DIR)" \
		--opt "build-arg:IKNITE_REPO_URL=https://static.iknite.app/$(IKNITE_REPO_NAME)/" \
		--opt "build-arg:IKNITE_VERSION=$(IKNITE_VERSION)" \
		$(CACHE_FLAG) \
		--output "type=docker,dest=-,name=$(IKNITE_CICONTAINER_IMAGE),push=false" | $(RUN_CONTAINER_CMD) load
	if [ "$(PUSH_IMAGES)" = "true" ]; then \
		$(RUN_CONTAINER_CMD) push "$(IKNITE_CICONTAINER_IMAGE)"; \
		if [ -n "$(IKNITE_CICONTAINER_IMAGE_ADDITIONAL_TAG)" ]; then \
			$(RUN_CONTAINER_CMD) tag "$(IKNITE_CICONTAINER_IMAGE)" "$(IKNITE_CICONTAINER_IMAGE_ADDITIONAL_TAG)"; \
			$(RUN_CONTAINER_CMD) push "$(IKNITE_CICONTAINER_IMAGE_ADDITIONAL_TAG)"; \
		fi; \
	fi
	@touch "$@"

.PHONY: cicontainer
cicontainer: $(IKNITE_CICONTAINER_IMAGE_MARKER)

.PHONY: check-argocd
check-argocd: $(IKNITE_CICONTAINER_IMAGE_MARKER) | check-prerequisites
	@echo "Checking ArgoCD application status using $(IKNITE_CICONTAINER_IMAGE) container..."
	$(RUN_CONTAINER_CMD) run --rm \
		-v "$(ROOT_DIR):/workspace" \
		-v "$(HOME)/.config/sops:/root/.config/sops:ro" \
		-v "$(HOME)/.config/incus:/root/.config/incus:ro" \
		-e "VM_STACK=$(VM_STACK)" \
		$(IKNITE_CICONTAINER_IMAGE) \
		/workspace/test/e2e/argocd-checker.sh

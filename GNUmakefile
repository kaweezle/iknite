# Makefile for building Iknite packages, rootfs images, and VM images.
# cSpell: words gsub rootfull chainguard apkindex doas vhdx apks covermode coverprofile checkmake
SHELL := /bin/sh

ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
ARCH ?= $(shell uname -m)
KUBERNETES_VERSION ?= $(shell grep 'k8s.io/kubernetes' "$(ROOT_DIR)/go.mod" | awk '{gsub(/^v/,"",$$2); print $$2;}')
KEY_NAME ?= kaweezle-devel@kaweezle.com-c9d89864.rsa
IKNITE_REPO_NAME ?= test
ROOTLESS ?= false
ROOTFULL ?= true
BUILDKIT_NAMESPACE ?= k8s.io
CACHE_FLAG ?= --no-cache
SNAPSHOT ?= --snapshot

SUDO_CMD := $(shell \
	if [ "$(ROOTLESS)" = "true" ] || [ "$$(id -u)" -eq 0 ]; then \
		echo ""; \
	elif command -v doas >/dev/null 2>&1; then \
		echo doas; \
	elif command -v sudo >/dev/null 2>&1; then \
		echo sudo; \
	else \
		echo ""; \
	fi)

DIST_DIR := dist
BUILD_DIR := build
KARMAFUN_LATEST_VERSION := $(shell curl --silent https://api.github.com/repos/karmafun/karmafun/releases/latest | jq -r .tag_name)
# IKNITE_VERSION := $(shell jq -Mr ".version" "$(DIST_DIR)/metadata.json" 2>/dev/null)
# Exact tag match of current version + 1 patch, with -devel suffix (e.g. v0.1.0 -> v0.1.1-devel)
IKNITE_VERSION := $(shell git describe --exact-match --tags --match="v[0-9]*" 2>/dev/null || (git describe --abbrev=0 --tags --match="v[0-9]*" | awk -F'.' '{ printf "%s.%s.%s-devel",$$1,$$2,$$3+1;}'))
IKNITE_NUMBER_VERSION := $(IKNITE_VERSION:v%=%)

# Base image for rootfs, containing only the APK packages without preloaded images
IKNITE_ROOTFS_BASE := iknite-rootfs-base:$(IKNITE_VERSION)
IKNITE_ROOTFS_BASE_MARKER = $(DIST_DIR)/iknite-rootfs-base_$(IKNITE_VERSION).marker
IKNITE_ROOTFS_BASE_SOURCES := $(wildcard $(ROOT_DIR)/packaging/rootfs/base/*)

# Final rootfs image with preloaded Kubernetes images
IKNITE_ROOTFS_IMAGE := ghcr.io/kaweezle/iknite/iknite:$(IKNITE_VERSION)-$(KUBERNETES_VERSION)
IKNITE_ROOTFS_IMAGE_MARKER = $(DIST_DIR)/iknite_$(IKNITE_VERSION)-$(KUBERNETES_VERSION).marker
IKNITE_ROOTFS_SOURCES := $(wildcard $(ROOT_DIR)/packaging/rootfs/with-images/*)

ROOTFS_NAME := iknite-$(IKNITE_NUMBER_VERSION)-$(KUBERNETES_VERSION).rootfs.tar.gz
ROOTFS_PATH := $(DIST_DIR)/$(ROOTFS_NAME)

# Package file names
KARMAFUN_PACKAGE := karmafun-$(patsubst v%,%,$(KARMAFUN_LATEST_VERSION)).$(ARCH).apk
IKNITE_PACKAGE := iknite-$(IKNITE_NUMBER_VERSION).$(ARCH).apk
IKNITE_IMAGES_PACKAGE := iknite-images-$(KUBERNETES_VERSION).$(ARCH).apk

KUSTOMIZATION_FILES = $(wildcard $(ROOT_DIR)/packaging/apk/iknite/iknite.d/**/*)
GOLANG_FILES = $(shell find "$(ROOT_DIR)/cmd" "$(ROOT_DIR)/pkg" -type f -name '*.go' ! -name '*_test.go')
APK_FILES = $(wildcard $(ROOT_DIR)/packaging/apk/iknite/**/*)
APK_INDEX_FILE = $(DIST_DIR)/repo/$(ARCH)/APKINDEX.tar.gz

IKNITE_ROOTFS_CONTAINER_MARKER = $(DIST_DIR)/iknite-rootfs-container_$(IKNITE_VERSION)-$(KUBERNETES_VERSION).marker

IKNITE_VM_IMAGE_QCOW2 = $(DIST_DIR)/images/iknite-vm.$(IKNITE_NUMBER_VERSION)-$(KUBERNETES_VERSION).qcow2
IKNITE_VM_IMAGE_VHDX = $(DIST_DIR)/images/iknite-vm.$(IKNITE_NUMBER_VERSION)-$(KUBERNETES_VERSION).vhdx

IMAGES_ID = $(shell $(SUDO_CMD) nerdctl -n $(BUILDKIT_NAMESPACE) image ls -q --filter "label=org.opencontainers.image.source=https://github.com/kaweezle/iknite.git" | sort -u)

BUILD_VM_IMAGE_SCRIPTS = $(ROOT_DIR)/packaging/scripts/build-vm-image.sh $(ROOT_DIR)/packaging/scripts/configure-vm-image.sh

.PHONY: help all goreleaser rootfs-base-image images-apk rootfs-container rootfs rootfs-image fetch-karmafun apk-repo upload-repo vm-image clean check-prerequisites test

help: # ignore checkmake
	@echo "Iknite build targets"
	@echo ""
	@echo "Step targets:"
	@echo "  make goreleaser         Build iknite package (goreleaser)"
	@echo "  make fetch-karmafun     Download latest karmafun APK into dist/"
	@echo "  make images-apk         Build iknite-images APK"
	@echo "  make apk-repo           Create APK repository in dist/repo"
	@echo "  make upload-repo        Upload APK repository with terragrunt"
	@echo "  make rootfs-base-image  Build rootfs base image"
	@echo "  make rootfs-container   Add preloaded images into rootfs container"
	@echo "  make rootfs             Build rootfs"
	@echo "  make rootfs-image       Build final rootfs image"
	@echo "  make vm-image           Build VM images (qcow2, vhdx)"
	@echo "  make clean              Remove build artifacts and temp container"
	@echo "  make all                Run full pipeline"
	@echo ""
	@echo "File targets (examples):"
	@echo "  make dist/iknite-<version>.$(ARCH).apk"
	@echo "  make dist/iknite-images-<k8s-version>.$(ARCH).apk"
	@echo "  make dist/karmafun-<version>.$(ARCH).apk"
	@echo ""
	@echo "Common variables (override with VAR=value):"
	@echo "  ARCH=$(ARCH)"
	@echo "  KUBERNETES_VERSION=$(KUBERNETES_VERSION)"
	@echo "  IKNITE_REPO_NAME=$(IKNITE_REPO_NAME)"
	@echo "  ROOTLESS=$(ROOTLESS)"
	@echo "  CACHE_FLAG=$(CACHE_FLAG)"
	@echo "  SNAPSHOT=$(SNAPSHOT)"

all: goreleaser fetch-karmafun images-apk apk-repo rootfs-base-image rootfs-container rootfs rootfs-image vm-image

print-% : ; $(info $* is a $(flavor $*) variable set to [$($*)]) @true

check-prerequisites:
	@mkdir -p "$(BUILD_DIR)" "$(DIST_DIR)"
	@command -v goreleaser >/dev/null 2>&1 || { echo "Error: goreleaser is not installed"; exit 1; }
	@test -f "$(ROOT_DIR)/$(KEY_NAME)" || { echo "Error: signing key '$(KEY_NAME)' is not present"; exit 1; }
	@command -v nerdctl >/dev/null 2>&1 || { echo "Error: nerdctl is not installed"; exit 1; }
	@command -v buildctl >/dev/null 2>&1 || { echo "Error: buildctl is not installed"; exit 1; }
	@if [ "$(ROOTLESS)" = "true" ]; then \
		nerdctl info >/dev/null 2>&1 || { echo "Error: containerd is not running (rootless)"; exit 1; }; \
	else \
		$(SUDO_CMD) nerdctl info >/dev/null 2>&1 || { echo "Error: containerd is not running"; exit 1; }; \
	fi
	@if [ "$(ROOTLESS)" = "true" ] && [ "$(ROOTFULL)" = "false" ]; then \
		if command -v systemctl >/dev/null 2>&1 && systemctl --version >/dev/null 2>&1; then \
			systemctl --user is-active --quiet buildkit || { echo "Error: buildkit user service is not running"; exit 1; }; \
		elif command -v rc-service >/dev/null 2>&1; then \
			[ -S "/run/user/$$(id -u)/buildkit/buildkitd.sock" ] || [ -S "$$HOME/.local/share/buildkit/buildkitd.sock" ] || { echo "Error: buildkit is not running"; exit 1; }; \
		elif [ -z "$$BUILDKIT_HOST" ]; then \
			echo "Error: BUILDKIT_HOST is not set"; exit 1; \
		fi; \
	else \
		[ -f "/run/buildkitd.pid" ] || pgrep -x buildkitd >/dev/null 2>&1 || { echo "Error: buildkit is not available"; exit 1; }; \
	fi

# File-based package targets in dist/
$(DIST_DIR)/$(IKNITE_PACKAGE) $(DIST_DIR)/metadata.json $(DIST_DIR)/iknite_linux_amd64_v1/iknite &: $(GOLANG_FILES) $(APK_FILES) go.mod .goreleaser.yaml | check-prerequisites
	goreleaser release --skip=publish $(SNAPSHOT) --clean

$(DIST_DIR)/$(KARMAFUN_PACKAGE): | check-prerequisites
	@echo "Latest karmafun version is $(KARMAFUN_LATEST_VERSION)"
	curl -o $@ -L "https://github.com/karmafun/karmafun/releases/download/$(KARMAFUN_LATEST_VERSION)/$(KARMAFUN_PACKAGE)"

$(DIST_DIR)/$(IKNITE_IMAGES_PACKAGE): $(DIST_DIR)/iknite_linux_amd64_v1/iknite $(KUSTOMIZATION_FILES) | check-prerequisites
	BUILD_DIR_SUFFIX="build/apk/iknite-images"; \
	BUILD_DIR="$(ROOT_DIR)/$$BUILD_DIR_SUFFIX"; \
	rm -rf "$$BUILD_DIR"; \
	mkdir -p "$$BUILD_DIR"; \
	echo "KUBERNETES_VERSION: $(KUBERNETES_VERSION)" > "$$BUILD_DIR/.env"; \
	"$(DIST_DIR)/iknite_linux_amd64_v1/iknite" kustomize -d "$(ROOT_DIR)/packaging/apk/iknite/iknite.d" print | grep image: | awk '{ print $$2; }' > "$$BUILD_DIR/image-list.txt"; \
	"$(DIST_DIR)/iknite_linux_amd64_v1/iknite" info images >> "$$BUILD_DIR/image-list.txt"; \
	sort -u "$$BUILD_DIR/image-list.txt" -o "$$BUILD_DIR/image-list.txt"; \
	$(SUDO_CMD) nerdctl run --privileged --rm -v "$(ROOT_DIR):/work" cgr.dev/chainguard/melange \
		build packaging/apk/iknite-images/iknite-images.yaml \
		--arch "$(ARCH)" \
		--vars-file "$$BUILD_DIR_SUFFIX/.env" \
		--source-dir "$$BUILD_DIR_SUFFIX" \
		--out-dir "dist" \
		--signing-key $(KEY_NAME) \
		--generate-index=false; \
	if [ "$(ROOTLESS)" = "false" ]; then \
		$(SUDO_CMD) chown -R "$$(id -u):$$(id -g)" "$(DIST_DIR)"; \
	fi; \
	(cd "$(DIST_DIR)/$(ARCH)" && for f in *.apk; do mv "$$f" "../$$(echo "$$f" | sed 's/\-r0.apk$$//').$(ARCH).apk"; done); \
	rmdir "$(DIST_DIR)/$(ARCH)/"


goreleaser: $(DIST_DIR)/$(IKNITE_PACKAGE) $(DIST_DIR)/metadata.json $(DIST_DIR)/iknite_linux_amd64_v1/iknite

fetch-karmafun: $(DIST_DIR)/$(KARMAFUN_PACKAGE)

images-apk: $(DIST_DIR)/$(IKNITE_IMAGES_PACKAGE)

$(APK_INDEX_FILE): $(DIST_DIR)/$(KARMAFUN_PACKAGE) $(DIST_DIR)/$(IKNITE_PACKAGE) $(DIST_DIR)/$(IKNITE_IMAGES_PACKAGE) | check-prerequisites
	rm -rf "$(DIST_DIR)/repo"
	mkdir -p "$(DIST_DIR)/repo"
	INPUT_APK_FILES="$(DIST_DIR)/*.apk" \
	INPUT_DESTINATION="$(DIST_DIR)/repo" \
	INPUT_SIGNATURE_KEY_NAME="$(KEY_NAME)" \
	INPUT_SIGNATURE_KEY="$$(cat "$(ROOT_DIR)/$(KEY_NAME)")" \
	GITHUB_WORKSPACE="$(ROOT_DIR)" \
	"$(ROOT_DIR)/.github/actions/make-apkindex/entrypoint.sh"

apk-repo: $(APK_INDEX_FILE)

upload-repo: $(APK_INDEX_FILE) | check-prerequisites
	cd "$(ROOT_DIR)/deploy/iac/iknite/$(IKNITE_REPO_NAME)repo" && terragrunt init && terragrunt apply -auto-approve


$(IKNITE_ROOTFS_BASE_MARKER): $(DIST_DIR)/$(KARMAFUN_PACKAGE) $(DIST_DIR)/$(IKNITE_PACKAGE) $(IKNITE_ROOTFS_BASE_SOURCES) | check-prerequisites
	BUILD_DIR_PATH="$(BUILD_DIR)/rootfs/base"; \
	rm -rf "$$BUILD_DIR_PATH"; \
	mkdir -p "$$BUILD_DIR_PATH"; \
	cp -r "$(ROOT_DIR)/packaging/rootfs/base/." "$$BUILD_DIR_PATH/"; \
	cp "$(DIST_DIR)/$(IKNITE_PACKAGE)" "$$BUILD_DIR_PATH/"; \
	cp "$(DIST_DIR)/$(KARMAFUN_PACKAGE)" "$$BUILD_DIR_PATH/"; \
	$(SUDO_CMD) buildctl build \
		--frontend dockerfile.v0 \
		--local "context=$$BUILD_DIR_PATH" \
		--local "dockerfile=$$BUILD_DIR_PATH" \
		--opt "build-arg:IKNITE_REPO_URL=https://static.iknite.app/$(IKNITE_REPO_NAME)/" \
		--opt "build-arg:IKNITE_VERSION=$(IKNITE_NUMBER_VERSION)" \
		$(CACHE_FLAG) \
		--output "type=image,name=$(IKNITE_ROOTFS_BASE),push=false"
	@touch "$@"

rootfs-base-image: $(IKNITE_ROOTFS_BASE_MARKER)

$(IKNITE_ROOTFS_CONTAINER_MARKER): $(DIST_DIR)/$(IKNITE_IMAGES_PACKAGE) $(IKNITE_ROOTFS_BASE_MARKER) | check-prerequisites
	$(SUDO_CMD) nerdctl -n $(BUILDKIT_NAMESPACE) rm -f iknite-rootfs 2>/dev/null || true
	$(SUDO_CMD) nerdctl -n $(BUILDKIT_NAMESPACE) run \
		--name iknite-rootfs \
		--device /dev/fuse --cap-add SYS_ADMIN \
		-v "$(realpath $(DIST_DIR)):/apks" \
		-e "IKNITE_IMAGES_PACKAGE=$(IKNITE_IMAGES_PACKAGE)" \
		"$(IKNITE_ROOTFS_BASE)" \
		/bin/sh -c 'apk --no-cache add /apks/$$IKNITE_IMAGES_PACKAGE; apk del iknite-images'
	@touch "$@"

rootfs-container: $(IKNITE_ROOTFS_CONTAINER_MARKER)

$(ROOTFS_PATH): $(IKNITE_ROOTFS_CONTAINER_MARKER) | check-prerequisites
	rm -f "$@" || true
	$(SUDO_CMD) nerdctl -n $(BUILDKIT_NAMESPACE) export iknite-rootfs | gzip > "$@"

rootfs: $(ROOTFS_PATH)

$(IKNITE_ROOTFS_IMAGE_MARKER): $(ROOTFS_PATH) $(IKNITE_ROOTFS_SOURCES) | check-prerequisites
	BUILD_DIR_PATH="$(BUILD_DIR)/rootfs/with-images"; \
	rm -rf "$$BUILD_DIR_PATH"; \
	mkdir -p "$$BUILD_DIR_PATH"; \
	cp -r "$(ROOT_DIR)/packaging/rootfs/with-images/." "$$BUILD_DIR_PATH/"; \
	cp "$(DIST_DIR)/$(ROOTFS_NAME)" "$$BUILD_DIR_PATH/$(ROOTFS_NAME)"; \
	$(SUDO_CMD) buildctl build \
		--frontend dockerfile.v0 \
		--local "context=$$BUILD_DIR_PATH" \
		--local "dockerfile=$$BUILD_DIR_PATH" \
		--opt "build-arg:IKNITE_VERSION=$(IKNITE_NUMBER_VERSION)" \
		--opt "build-arg:KUBERNETES_VERSION=$(KUBERNETES_VERSION)" \
		$(CACHE_FLAG) \
		--output "type=image,name=$(IKNITE_ROOTFS_IMAGE),push=false"
	@touch "$@"

rootfs-image: $(IKNITE_ROOTFS_IMAGE_MARKER)

$(IKNITE_VM_IMAGE_QCOW2) $(IKNITE_VM_IMAGE_VHDX) &: $(ROOTFS_PATH) $(BUILD_VM_IMAGE_SCRIPTS) | check-prerequisites
	rm -f "$(dir $@)*" || true
	$(SUDO_CMD) "$(ROOT_DIR)/packaging/scripts/build-vm-image.sh"

vm-image: $(IKNITE_VM_IMAGE_QCOW2) $(IKNITE_VM_IMAGE_VHDX)

clean:
	rm -rf "$(BUILD_DIR)"
	rm -rf "$(DIST_DIR)"
	$(SUDO_CMD) nerdctl -n $(BUILDKIT_NAMESPACE) rm -f iknite-rootfs >/dev/null 2>&1 || true
	$(SUDO_CMD) buildctl prune >/dev/null 2>&1 || true
	$(SUDO_CMD) nerdctl -n $(BUILDKIT_NAMESPACE) image rm -f $(IMAGES_ID) >/dev/null 2>&1 || true

test: check-prerequisites
	@echo "Running tests..."
	go test -v -race -covermode=atomic -coverprofile=coverage.out ./...

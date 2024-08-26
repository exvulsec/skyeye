.PHONY: build
TARGET=skyeye

OUTPUT_DIR := ./bin
BUILD_DIR = ./build

VERSION      ?= $(shell git describe --tags --always --dirty)
REPO_URL := git@github.com:exvulsec/simulation.git

DOCKER_LABELS ?= git-describe="$(shell date -u +v%Y%m%d)-$(shell git describe --tags --always --dirty)"

# It's necessary to set this because some environments don't link sh -> bash.
export SHELL := /bin/bash

# It's necessary to set the errexit flags for the bash shell.
export SHELLOPTS := errexit

build: build-local

run:
	docker-compose up -d


# 定义目标操作系统和架构
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# 对于Docker镜像，我们默认使用linux/amd64
DOCKER_GOOS ?= linux
DOCKER_GOARCH ?= amd64

# Rust目标三元组映射
RUST_TARGET_$(GOOS)_$(GOARCH) ?= $(GOOS)-$(GOARCH)
RUST_TARGET_linux_amd64 := x86_64-unknown-linux-gnu
RUST_TARGET_linux_arm64 := aarch64-unknown-linux-gnu
RUST_TARGET_darwin_amd64 := x86_64-apple-darwin
RUST_TARGET_darwin_arm64 := aarch64-apple-darwin

RUST_TARGET ?= $(RUST_TARGET_$(GOOS)_$(GOARCH))
DOCKER_RUST_TARGET ?= $(RUST_TARGET_$(DOCKER_GOOS)_$(DOCKER_GOARCH))

build-rust-lib:
ifeq ($(shell [ -d simulation ] && echo yes),yes)
	cd simulation && git fetch origin && git rebase origin/main
else
	git clone $(REPO_URL)
endif
	cd simulation && cargo build -p simulation --release --target $(RUST_TARGET)
	mkdir -p ./lib
	cp -r simulation/target/$(RUST_TARGET)/release/libsimulation.* ./lib


build-local: build-rust-lib
	go mod download;                                                \
	CGO_ENABLED=1                                                   \
	GOOS=$(GOOS) GOARCH=$(GOARCH)                                   \
	go build -v -o $(OUTPUT_DIR)/$(TARGET) main.go;

build-image:
	$(MAKE) build-local GOOS=$(DOCKER_GOOS) GOARCH=$(DOCKER_GOARCH) RUST_TARGET=$(DOCKER_RUST_TARGET)
	image=$(IMAGE_PREFIX)$(TARGET)$(IMAGE_SUFFIX);                  \
	docker build                                                    \
	  --platform=$(DOCKER_GOOS)/$(DOCKER_GOARCH)                    \
	  -t $${image}:$(VERSION)                                       \
	  --label $(DOCKER_LABELS)                                      \
	  -f $(BUILD_DIR)/Dockerfile .;                                 \
	sed -i                                                          \
	  "/\ \ \ \ image: $${image}:*/c\ \ \ \ image: $${image}:$(VERSION)"        \
	  $(PWD)/docker-compose.yaml;
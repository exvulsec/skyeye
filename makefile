.PHONY: build
TARGET=etl

OUTPUT_DIR := ./bin
BUILD_DIR = ./build

VERSION      ?= $(shell git describe --tags --always --dirty)

DOCKER_LABELS ?= git-describe="$(shell date -u +v%Y%m%d)-$(shell git describe --tags --always --dirty)"

# It's necessary to set this because some environments don't link sh -> bash.
export SHELL := /bin/bash

# It's necessary to set the errexit flags for the bash shell.
export SHELLOPTS := errexit

build: build-local


build-local:
	go build -v -o $(OUTPUT_DIR)/$(TARGET) main.go;

build-linux:
	@docker run --rm -it                                              \
	  --platform=linux/amd64                                          \
	  -v $(PWD):/go/src/$(TARGET)                                     \
	  -w /go/src/$(TARGET)                                            \
	  -e GOPROXY=https://goproxy.io,direct                            \
	  -e GOPATH=/go                                                   \
	  -e SHELLOPTS="$(SHELLOPTS)"                                     \
	  golang:1.20.2-buster                                            \
	    /bin/bash -c 'go mod download;                                \
	    CGO_ENABLED=0 GOOS=linux GOARCH=amd64                         \
	    go build -v -o $(OUTPUT_DIR)/$(TARGET) main.go;'

build-image: build-linux
	image=$(IMAGE_PREFIX)$(TARGET)$(IMAGE_SUFFIX);                              \
	docker build                                                                \
	  --platform=linux/amd64                                                    \
	  -t $${image}:$(VERSION)                                                   \
	  --label $(DOCKER_LABELS)                                                  \
	  -f $(BUILD_DIR)/Dockerfile .;                                             \
	sed -i                                                                      \
	  "/\ \ \ \ image: $${image}:*/c\ \ \ \ image: $${image}:$(VERSION)"        \
	  $(PWD)/docker-compose.yaml;


run:
	docker-compose up -d

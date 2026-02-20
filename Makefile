.PHONY: all build build-all clean run test deps deploy-fly docker-build docker-push help

BINARY_NAME=oci-explorer
GIT_DESC := $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//')
VERSION ?= $(if $(findstring dirty,$(GIT_DESC)),$(shell echo $(GIT_DESC) | sed 's/-dirty//')-dev+$(shell git rev-parse --short HEAD),$(GIT_DESC))
BUILD_DIR=build
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"
PLATFORMS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
DOCKER_IMAGE=ghcr.io/hkolvenbach/oci-explorer
DOCKER_PLATFORMS=linux/amd64,linux/arm64

all: deps build

deps:
	go mod download
	go mod tidy

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(LDFLAGS) .

build-all: deps
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		echo "Building for $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch go build -o $(BUILD_DIR)/$(BINARY_NAME)-$$os-$$arch $(LDFLAGS) . || exit 1; \
	done
	@echo "Build complete! Binaries are in $(BUILD_DIR)/"

run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

test:
	go test -v ./...

clean:
	go clean
	rm -rf $(BUILD_DIR)

release: build-all
	@mkdir -p $(BUILD_DIR)/release
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		tar -czvf $(BUILD_DIR)/release/$(BINARY_NAME)-$(VERSION)-$$os-$$arch.tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-$$os-$$arch; \
	done
	@cd $(BUILD_DIR)/release && shasum -a 256 *.tar.gz > checksums.txt
	@echo "Release archives created in $(BUILD_DIR)/release/"

docker-build:
	@mkdir -p $(BUILD_DIR)
	docker buildx build --platform $(DOCKER_PLATFORMS) \
		-t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest \
		--build-arg VERSION=$(VERSION) \
		--output type=oci,dest=$(BUILD_DIR)/$(BINARY_NAME)-$(VERSION).tar .

docker-push:
	docker buildx build --platform $(DOCKER_PLATFORMS) \
		-t $(DOCKER_IMAGE):$(VERSION) -t $(DOCKER_IMAGE):latest \
		--build-arg VERSION=$(VERSION) --push .

deploy-fly:
	@which fly > /dev/null || (echo "Error: flyctl not found. Install from https://fly.io/docs/hands-on/install-flyctl/" && exit 1)
	fly deploy

help:
	@echo "OCI Image Explorer - Build Commands"
	@echo ""
	@echo "Usage:"
	@echo "  make deps                    Download and tidy dependencies"
	@echo "  make build                   Build for current platform"
	@echo "  make build-all               Build for all platforms (linux, darwin)"
	@echo "  make run                     Build and run the application"
	@echo "  make test                    Run tests"
	@echo "  make clean                   Clean build artifacts"
	@echo "  make release                 Create release archives with checksums"
	@echo "  make docker-build            Build multi-arch Docker image to OCI tarball"
	@echo "  make docker-push             Build and push multi-arch Docker image"
	@echo "  make deploy-fly              Deploy to Fly.io"

.PHONY: all build build-all clean run test deps deploy-fly help

BINARY_NAME=oci-explorer
VERSION?=0.1.0
BUILD_DIR=build
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"
PLATFORMS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

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
	@echo "Release archives created in $(BUILD_DIR)/release/"

deploy-fly:
	@which fly > /dev/null || (echo "Error: flyctl not found. Install from https://fly.io/docs/hands-on/install-flyctl/" && exit 1)
	fly deploy

help:
	@echo "OCI Image Explorer - Build Commands"
	@echo ""
	@echo "Usage:"
	@echo "  make deps               Download and tidy dependencies"
	@echo "  make build              Build for current platform"
	@echo "  make build-all          Build for all platforms (linux, darwin)"
	@echo "  make run                Build and run the application"
	@echo "  make test               Run tests"
	@echo "  make clean              Clean build artifacts"
	@echo "  make release            Create release archives for all platforms"
	@echo "  make deploy-fly         Deploy to Fly.io"

# Binary name
BINARY_NAME=zx
OUTPUT_DIR=build
PACKAGE_DIR=release

# Default build and package all
all: build package

# Build all platforms
build: build-linux build-windows build-macos build-linux-arm64 build-macos-arm64

# Current platform build
native:
	go build -o $(BINARY_NAME) main.go

# Platform builds
build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(OUTPUT_DIR)/$(BINARY_NAME)-linux main.go

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -o $(OUTPUT_DIR)/$(BINARY_NAME)-linux-arm64 main.go

build-windows:
	GOOS=windows GOARCH=amd64 go build -o $(OUTPUT_DIR)/$(BINARY_NAME).exe main.go

build-macos:
	GOOS=darwin GOARCH=amd64 go build -o $(OUTPUT_DIR)/$(BINARY_NAME)-macos main.go

build-macos-arm64:
	GOOS=darwin GOARCH=arm64 go build -o $(OUTPUT_DIR)/$(BINARY_NAME)-macos-arm64 main.go

# Archive packages
package: clean-packages package-linux package-linux-arm64 package-windows package-macos package-macos-arm64

package-linux:
	tar -czf $(PACKAGE_DIR)/$(BINARY_NAME)-linux.tar.gz -C $(OUTPUT_DIR) $(BINARY_NAME)-linux

package-linux-arm64:
	tar -czf $(PACKAGE_DIR)/$(BINARY_NAME)-linux-arm64.tar.gz -C $(OUTPUT_DIR) $(BINARY_NAME)-linux-arm64

package-macos:
	tar -czf $(PACKAGE_DIR)/$(BINARY_NAME)-macos.tar.gz -C $(OUTPUT_DIR) $(BINARY_NAME)-macos

package-macos-arm64:
	tar -czf $(PACKAGE_DIR)/$(BINARY_NAME)-macos-arm64.tar.gz -C $(OUTPUT_DIR) $(BINARY_NAME)-macos-arm64

package-windows:
	cd $(OUTPUT_DIR) && zip ../$(PACKAGE_DIR)/$(BINARY_NAME)-windows.zip $(BINARY_NAME).exe

# Run app
run:
	go run main.go

# Install to GOBIN
install:
	go install ./...

# Tests
test:
	go test ./...

# Clean build output
clean:
	rm -rf $(OUTPUT_DIR)/ $(BINARY_NAME)

# Clean packaged archives
clean-packages:
	rm -rf $(PACKAGE_DIR)/
	mkdir -p $(PACKAGE_DIR)

# Format code
fmt:
	go fmt ./...

# Lint (requires golangci-lint)
lint:
	golangci-lint run

# Show help
help:
	@echo "Makefile for $(BINARY_NAME)"
	@echo ""
	@echo "Targets:"
	@echo "  make build             - Build all platform binaries"
	@echo "  make package           - Create .tar.gz/.zip archives in 'release/'"
	@echo "  make native            - Build for current OS"
	@echo "  make clean             - Remove all builds"
	@echo "  make clean-packages    - Clear release packages"

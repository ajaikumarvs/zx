# Binary name
BINARY_NAME=zx
OUTPUT_DIR=build

# Default: Cross-compile for all targets
build: build-linux build-windows build-macos build-linux-arm64 build-macos-arm64

# Build for current platform
native:
	go build -o $(BINARY_NAME) main.go

# Linux x86_64
build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(OUTPUT_DIR)/$(BINARY_NAME)-linux main.go

# Linux ARM64
build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build -o $(OUTPUT_DIR)/$(BINARY_NAME)-linux-arm64 main.go

# Windows x86_64
build-windows:
	GOOS=windows GOARCH=amd64 go build -o $(OUTPUT_DIR)/$(BINARY_NAME).exe main.go

# macOS x86_64
build-macos:
	GOOS=darwin GOARCH=amd64 go build -o $(OUTPUT_DIR)/$(BINARY_NAME)-macos main.go

# macOS ARM64 (M1/M2)
build-macos-arm64:
	GOOS=darwin GOARCH=arm64 go build -o $(OUTPUT_DIR)/$(BINARY_NAME)-macos-arm64 main.go

# Run locally
run:
	go run main.go

# Install to GOBIN
install:
	go install ./...

# Run unit tests
test:
	go test ./...

# Clean compiled binaries
clean:
	rm -rf $(OUTPUT_DIR)/
	rm -f $(BINARY_NAME)

# Format source code
fmt:
	go fmt ./...

# Run linter (if installed)
lint:
	golangci-lint run

# Print help
help:
	@echo "Makefile for $(BINARY_NAME)"
	@echo ""
	@echo "Common targets:"
	@echo "  make build              - Cross-compile for all targets"
	@echo "  make native             - Build for current OS/Arch"
	@echo "  make build-linux        - Linux x86_64"
	@echo "  make build-linux-arm64  - Linux ARM64"
	@echo "  make build-macos        - macOS x86_64"
	@echo "  make build-macos-arm64  - macOS ARM64"
	@echo "  make build-windows      - Windows x86_64"
	@echo "  make install            - Install binary to \$GOBIN"
	@echo "  make test               - Run tests"
	@echo "  make clean              - Clean build output"

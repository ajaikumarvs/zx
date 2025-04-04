# Binary name and folders
BINARY_NAME=zx
OUTPUT_DIR=build
PACKAGE_DIR=release
CHECKSUM_FILE=$(PACKAGE_DIR)/checksums.txt

# Default target: build, package, checksum
all: build package checksums

# Cross-platform builds
build: build-linux build-windows build-macos build-linux-arm64 build-macos-arm64

native:
	go build -o $(BINARY_NAME) main.go

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

# Archive builds
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

#Generate SHA256 checksums
checksums:
	cd $(PACKAGE_DIR) && sha256sum * > $(CHECKSUM_FILE)
	@echo "Checksums written to $(CHECKSUM_FILE)"

#GitHub release upload
#Example: make release VERSION=1.0.0 PRERELEASE=true
release: all checksums
	@echo "Creating GitHub release v$(VERSION)"
	@CHANGELOG=$$(git log --pretty=format:'* %s (%an)' $$(git describe --tags --abbrev=0)..HEAD); \
	if [ "$(PRERELEASE)" = "true" ]; then \
		gh release create v$(VERSION) --title "v$(VERSION)" --notes "$$CHANGELOG" --prerelease $(PACKAGE_DIR)/*; \
	else \
		gh release create v$(VERSION) --title "v$(VERSION)" --notes "$$CHANGELOG" $(PACKAGE_DIR)/*; \
	fi


# Install locally
install:
	go install ./...

# Tests
test:
	go test ./...

# Clean
clean:
	rm -rf $(OUTPUT_DIR)/
	rm -f $(BINARY_NAME)

clean-packages:
	rm -rf $(PACKAGE_DIR)/
	mkdir -p $(PACKAGE_DIR)

# Formatting / Linting
fmt:
	go fmt ./...

lint:
	golangci-lint run

# Help
help:
	@echo "Makefile for $(BINARY_NAME)"
	@echo "Usage:"
	@echo "  make build            - Build binaries for all platforms"
	@echo "  make package          - Create tar.gz/zip archives"
	@echo "  make checksums        - Generate SHA256 checksums"
	@echo "  make release VERSION=1.0.0 - Publish GitHub release"

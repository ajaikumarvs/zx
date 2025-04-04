# Binary and version
BINARY_NAME = zx
VERSION ?= dev
OUTPUT_DIR = build
PACKAGE_DIR = release
CHECKSUM_FILE = $(PACKAGE_DIR)/checksums.txt
OUTPUT_NAME = $(BINARY_NAME)-$(VERSION)

# Default target
all: build package checksums

# Cross-platform builds
build: build-linux build-linux-arm64 build-windows build-macos build-macos-arm64

native:
	go build -o $(BINARY_NAME) main.go

build-linux:
	mkdir -p $(OUTPUT_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(OUTPUT_DIR)/$(OUTPUT_NAME)-linux main.go

build-linux-arm64:
	mkdir -p $(OUTPUT_DIR)
	GOOS=linux GOARCH=arm64 go build -o $(OUTPUT_DIR)/$(OUTPUT_NAME)-linux-arm64 main.go

build-windows:
	mkdir -p $(OUTPUT_DIR)
	GOOS=windows GOARCH=amd64 go build -o $(OUTPUT_DIR)/$(OUTPUT_NAME).exe main.go

build-macos:
	mkdir -p $(OUTPUT_DIR)
	GOOS=darwin GOARCH=amd64 go build -o $(OUTPUT_DIR)/$(OUTPUT_NAME)-macos main.go

build-macos-arm64:
	mkdir -p $(OUTPUT_DIR)
	GOOS=darwin GOARCH=arm64 go build -o $(OUTPUT_DIR)/$(OUTPUT_NAME)-macos-arm64 main.go

# DEB and RPM packaging
DEB_NAME = $(PACKAGE_DIR)/$(OUTPUT_NAME)-linux.deb
RPM_NAME = $(PACKAGE_DIR)/$(OUTPUT_NAME)-linux.rpm

packages: $(DEB_NAME) $(RPM_NAME)

$(DEB_NAME): $(OUTPUT_DIR)/$(OUTPUT_NAME)-linux
	mkdir -p tmp/DEBIAN
	echo "Package: $(BINARY_NAME)\nVersion: $(VERSION)\nSection: base\nPriority: optional\nArchitecture: amd64\nMaintainer: You <you@example.com>\nDescription: Fast grep-like CLI" > tmp/DEBIAN/control
	mkdir -p tmp/usr/local/bin
	cp $(OUTPUT_DIR)/$(OUTPUT_NAME)-linux tmp/usr/local/bin/$(BINARY_NAME)
	dpkg-deb --build tmp $(DEB_NAME)
	rm -rf tmp

$(RPM_NAME): $(OUTPUT_DIR)/$(OUTPUT_NAME)-linux
	fpm -s dir -t rpm -n $(BINARY_NAME) -v $(VERSION) --prefix /usr/local/bin $(OUTPUT_DIR)/$(OUTPUT_NAME)-linux
	mv *.rpm $(RPM_NAME)

# Packaging archives
package: clean-packages package-linux package-linux-arm64 package-windows package-macos package-macos-arm64

package-linux:
	tar -czf $(PACKAGE_DIR)/$(OUTPUT_NAME)-linux.tar.gz -C $(OUTPUT_DIR) $(OUTPUT_NAME)-linux

package-linux-arm64:
	tar -czf $(PACKAGE_DIR)/$(OUTPUT_NAME)-linux-arm64.tar.gz -C $(OUTPUT_DIR) $(OUTPUT_NAME)-linux-arm64

package-macos:
	tar -czf $(PACKAGE_DIR)/$(OUTPUT_NAME)-macos.tar.gz -C $(OUTPUT_DIR) $(OUTPUT_NAME)-macos

package-macos-arm64:
	tar -czf $(PACKAGE_DIR)/$(OUTPUT_NAME)-macos-arm64.tar.gz -C $(OUTPUT_DIR) $(OUTPUT_NAME)-macos-arm64

package-windows:
	cd $(OUTPUT_DIR) && zip ../$(PACKAGE_DIR)/$(OUTPUT_NAME)-windows.zip $(OUTPUT_NAME).exe

# Checksums (portable)
checksums:
	cd $(PACKAGE_DIR) && \
	( command -v sha256sum >/dev/null 2>&1 && sha256sum * > $(CHECKSUM_FILE) || shasum -a 256 * > $(CHECKSUM_FILE) )
	@echo "Checksums written to $(CHECKSUM_FILE)"

# Release with changelog
release: check-version all packages checksums
	@echo "Creating GitHub release v$(VERSION)"
	@CHANGELOG=$$(git log --pretty=format:'* %s (%an)' $$(git describe --tags --abbrev=0)..HEAD); \
	if [ "$(PRERELEASE)" = "true" ]; then \
		gh release create v$(VERSION) --title "v$(VERSION)" --notes "$$CHANGELOG" --prerelease $(PACKAGE_DIR)/*; \
	else \
		gh release create v$(VERSION) --title "v$(VERSION)" --notes "$$CHANGELOG" $(PACKAGE_DIR)/*; \
	fi

check-version:
ifndef VERSION
	$(error VERSION is not set. Usage: make release VERSION=x.y.z)
endif

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

# Formatting and linting
fmt:
	go fmt ./...

lint:
	golangci-lint run

# Help
help:
	@echo "Makefile for $(BINARY_NAME)"
	@echo "Usage:"
	@echo "  make build             - Build for all platforms"
	@echo "  make native            - Build for host system"
	@echo "  make package           - Archive builds"
	@echo "  make checksums         - Generate SHA256 checksums"
	@echo "  make release VERSION=x.y.z - Publish GitHub release"
	@echo "  make install           - Install locally"
	@echo "  make test              - Run tests"
	@echo "  make fmt / lint        - Format or lint code"
	@echo "  make clean             - Clean build artifacts"

.PHONY: all build native install test fmt lint clean clean-packages help checksums release package packages check-version

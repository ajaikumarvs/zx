# Binary name
BINARY_NAME=zx

# Default target
all: build

# Build the Go binary
build:
	go build -o $(BINARY_NAME) main.go

# Run the project
run:
	go run main.go

# Install the binary globally
install:
	go install ./...

# Run tests
test:
	go test ./...

# Clean up build artifacts
clean:
	rm -f $(BINARY_NAME)

# Format code
fmt:
	go fmt ./...

# Lint (requires golangci-lint installed)
lint:
	golangci-lint run

# Display help
help:
	@echo "Makefile for $(BINARY_NAME)"
	@echo "Usage:"
	@echo "  make build     - Build the binary"
	@echo "  make run       - Run the app"
	@echo "  make install   - Install globally"
	@echo "  make test      - Run tests"
	@echo "  make clean     - Remove binary"
	@echo "  make fmt       - Format code"
	@echo "  make lint      - Lint code (requires golangci-lint)"

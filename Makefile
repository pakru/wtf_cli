.PHONY: all build test clean run fmt vet lint check help version release-local

# Version information
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X 'main.version=$(VERSION)' -X 'main.commit=$(GIT_COMMIT)' -X 'main.date=$(BUILD_DATE)'

# Default target
all: check build test

# Build the binary
build:
	go build -ldflags "$(LDFLAGS)" -o wtf_cli ./cmd/wtf_cli

# Run all tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f wtf_cli

# Build and run
run: build
	./wtf_cli

# Format all Go code
fmt:
	go fmt ./...

# Run go vet for static analysis
vet:
	go vet ./...

# Check for common issues (fmt, vet)
lint: fmt vet
	@echo "✓ Code formatting and vetting complete"

# Pre-commit check: format, vet, build, and test
check: fmt vet
	@echo "Running build..."
	@$(MAKE) build
	@echo "Running tests..."
	@$(MAKE) test
	@echo "✓ All checks passed!"

# Show current version
version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(GIT_COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"

# Build with release flags (for local testing)
release-local:
	go build -ldflags "$(LDFLAGS) -s -w" -trimpath -o wtf_cli ./cmd/wtf_cli
	@echo "✓ Release build complete"

# Show available targets
help:
	@echo "Available targets:"
	@echo "  make all           - Run checks, build, and test (default)"
	@echo "  make build         - Build the wtf_cli binary with version info"
	@echo "  make test          - Run all tests"
	@echo "  make clean         - Remove build artifacts"
	@echo "  make run           - Build and run the application"
	@echo "  make fmt           - Format all Go code"
	@echo "  make vet           - Run go vet static analysis"
	@echo "  make lint          - Run formatting and vetting"
	@echo "  make check         - Full pre-commit validation"
	@echo "  make version       - Show current version information"
	@echo "  make release-local - Build optimized release binary locally"
	@echo "  make help          - Show this help message"

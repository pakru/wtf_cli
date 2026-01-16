.PHONY: all build test clean run fmt vet lint check help

# Default target
all: check build test

# Build the binary
build:
	go build -o wtf_cli cmd/wtf_cli/main.go

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

# Show available targets
help:
	@echo "Available targets:"
	@echo "  make all      - Run checks, build, and test (default)"
	@echo "  make build    - Build the wtf_cli binary"
	@echo "  make test     - Run all tests"
	@echo "  make clean    - Remove build artifacts"
	@echo "  make run      - Build and run the application"
	@echo "  make fmt      - Format all Go code"
	@echo "  make vet      - Run go vet static analysis"
	@echo "  make lint     - Run formatting and vetting"
	@echo "  make check    - Full pre-commit validation"
	@echo "  make help     - Show this help message"

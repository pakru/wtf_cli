.PHONY: build test clean run install fmt vet lint help

# Default target
all: build

# Build the binary
build:
	@echo "Building wtf CLI..."
	go build -o wtf .

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f wtf coverage.out coverage.html

# Run the application
run: build
	@echo "Running wtf CLI..."
	./wtf

# Install the binary to GOPATH/bin
install:
	@echo "Installing wtf CLI..."
	go install .

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Run golangci-lint (if available)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

# Check for security vulnerabilities
security:
	@echo "Checking for security vulnerabilities..."
	@if command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "govulncheck not found. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	fi

# Development workflow: format, vet, test, build
dev: fmt vet test build

# CI workflow: tidy, format check, vet, test, build
ci: tidy fmt-check vet test build

# Check if code is formatted
fmt-check:
	@echo "Checking code formatting..."
	@if [ -n "$$(go fmt ./...)" ]; then \
		echo "Code is not formatted. Run 'make fmt' to fix."; \
		exit 1; \
	fi

# Help target
help:
	@echo "Available targets:"
	@echo "  build         - Build the wtf binary"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts"
	@echo "  run           - Build and run the application"
	@echo "  install       - Install binary to GOPATH/bin"
	@echo "  fmt           - Format code"
	@echo "  vet           - Run go vet"
	@echo "  lint          - Run golangci-lint"
	@echo "  tidy          - Tidy dependencies"
	@echo "  security      - Check for security vulnerabilities"
	@echo "  dev           - Development workflow (fmt, vet, test, build)"
	@echo "  ci            - CI workflow (tidy, fmt-check, vet, test, build)"
	@echo "  help          - Show this help message"

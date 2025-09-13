.PHONY: build test test-shell test-integration test-install test-coverage coverage-report coverage-html clean run install install-full uninstall fmt vet lint docker-build help

# Default target
all: build

# Build the binary
build:
	@echo "Building wtf CLI..."
	@mkdir -p build
	go build -o build/wtf .

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Run shell integration tests
test-shell:
	@echo "Running shell integration tests..."
	@bash shell/test_integration.sh

# Run comprehensive end-to-end shell integration test
test-shell-e2e:
	@echo "Running end-to-end shell integration test..."
	@bash scripts/test_shell_integration_e2e.sh

# Test the installation script
test-install:
	@echo "Testing installation script..."
	@bash scripts/test_installation.sh

# Run combined integration tests
test-integration: test test-shell-e2e
	@echo "All integration tests completed"

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	@echo "Coverage profile generated: coverage.out"

# Generate coverage report (text format)
coverage-report: test-coverage
	@echo "Generating coverage report..."
	@go tool cover -func=coverage.out
	@echo ""
	@echo "=== COVERAGE SUMMARY ==="
	@go tool cover -func=coverage.out | grep total | awk '{print "Total Coverage: " $$3}'

# Generate HTML coverage report
coverage-html: test-coverage
	@echo "Generating HTML coverage report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "HTML coverage report generated: coverage.html"
	@echo "Open coverage.html in your browser to view detailed coverage"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf build/ coverage.out coverage.html

# Run the application
run: build
	@echo "Running wtf CLI..."
	./build/wtf

# Install the binary to GOPATH/bin
install:
	@echo "Installing wtf CLI..."
	go install .

# Full installation with shell integration
install-full:
	@echo "Running full WTF CLI installation..."
	@./scripts/install.sh

# Uninstall WTF CLI
uninstall:
	@echo "Uninstalling WTF CLI..."
	@./scripts/install.sh uninstall

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

# Build Docker test image
docker-build: build
	@echo "Building Docker test image..."
	docker build -f docker/Dockerfile -t wtf-cli-test:latest .

# Development workflow: format, vet, test, build
dev: fmt vet test build

# Full development workflow with shell integration tests
dev-full: fmt vet test-integration build

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
	@echo "  test          - Run Go tests"
	@echo "  test-shell    - Run shell integration tests"
	@echo "  test-integration - Run all tests (Go + Shell)"
	@echo "  test-install  - Test the installation script"
	@echo "  test-coverage - Run tests with coverage profile"
	@echo "  coverage-report - Generate text coverage report"
	@echo "  coverage-html - Generate HTML coverage report"
	@echo "  clean         - Clean build artifacts"
	@echo "  run           - Build and run the application"
	@echo "  install       - Install binary to GOPATH/bin"
	@echo "  install-full  - Full installation with shell integration"
	@echo "  uninstall     - Uninstall WTF CLI completely"
	@echo "  fmt           - Format code"
	@echo "  vet           - Run go vet"
	@echo "  lint          - Run golangci-lint"
	@echo "  tidy          - Tidy dependencies"
	@echo "  security      - Check for security vulnerabilities"
	@echo "  docker-build  - Build Docker test image (requires binary to be built first)"
	@echo "  dev           - Development workflow (fmt, vet, test, build)"
	@echo "  dev-full      - Full development workflow with shell integration tests"
	@echo "  ci            - CI workflow (tidy, fmt-check, vet, test, build)"
	@echo "  help          - Show this help message"

# Makefile for systemd Go library package

.PHONY: test test-race test-verbose bench clean fmt vet lint install-tools check dev-check help

# Go parameters
GOCMD=go
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOVET=$(GOCMD) vet

# Test parameters
TEST_TIMEOUT=30s
BENCH_TIME=5s

all: check

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -timeout $(TEST_TIMEOUT) ./...

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	$(GOTEST) -race -timeout $(TEST_TIMEOUT) ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	$(GOTEST) -v -timeout $(TEST_TIMEOUT) ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -cover -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchtime=$(BENCH_TIME) ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# Vet code
vet:
	@echo "Vetting code..."
	$(GOVET) ./...

# Check for common Go mistakes
lint: install-tools
	@echo "Running golangci-lint..."
	golangci-lint run

# Clean artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f coverage.out coverage.html

# Tidy modules
tidy:
	@echo "Tidying modules..."
	$(GOMOD) tidy

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
# Install development tools
install-tools:
	@echo "Installing development tools..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin)

# Run all checks (useful for CI)
check: fmt vet test-race lint
	@echo "All checks passed!"

# Quick development check (faster than full check)
dev-check: fmt vet test
	@echo "Development checks passed!"

# Help target
help:
	@echo "Available targets:"
	@echo "  test         - Run tests"
	@echo "  test-race    - Run tests with race detection"
	@echo "  test-verbose - Run tests with verbose output"
	@echo "  test-coverage- Run tests with coverage report"
	@echo "  bench        - Run benchmarks"
	@echo "  fmt          - Format code"
	@echo "  vet          - Vet code"
	@echo "  lint         - Run golangci-lint"
	@echo "  clean        - Clean artifacts"
	@echo "  tidy         - Tidy modules"
	@echo "  deps         - Download dependencies"
	@echo "  install-tools- Install development tools"
	@echo "  check        - Run all checks (CI)"
	@echo "  dev-check    - Run quick development checks"
	@echo "  help         - Show this help message"

# Default target
.DEFAULT_GOAL := help

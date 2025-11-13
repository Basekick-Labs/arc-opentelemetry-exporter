.PHONY: all build test lint clean install-tools tidy

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Build parameters
BINARY_NAME=arc-exporter
OUTPUT_DIR=./bin

all: lint test build

build:
	@echo "Building Arc OpenTelemetry Exporter..."
	@mkdir -p $(OUTPUT_DIR)
	$(GOBUILD) -o $(OUTPUT_DIR)/$(BINARY_NAME) -v

test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

lint:
	@echo "Running linter..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed. Run 'make install-tools'"; exit 1; }
	golangci-lint run ./...

fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

tidy:
	@echo "Tidying go modules..."
	$(GOMOD) tidy

install-tools:
	@echo "Installing development tools..."
	@command -v golangci-lint >/dev/null 2>&1 || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v1.55.2
	@echo "Tools installed successfully"

clean:
	@echo "Cleaning..."
	@rm -rf $(OUTPUT_DIR)
	@rm -f coverage.out coverage.html

# Build with OCB (OpenTelemetry Collector Builder)
build-collector:
	@echo "Building custom collector with Arc exporter..."
	@command -v ocb >/dev/null 2>&1 || { echo "ocb not installed. Install from https://github.com/open-telemetry/opentelemetry-collector/releases"; exit 1; }
	ocb --config examples/builder-config.yaml

# Run example
run-example:
	@echo "Starting Arc and OTel Collector with Docker Compose..."
	cd examples && docker-compose up

# Stop example
stop-example:
	@echo "Stopping Docker Compose..."
	cd examples && docker-compose down

# Generate go.sum
gosum:
	$(GOMOD) download

help:
	@echo "Available targets:"
	@echo "  all              - Run lint, test, and build"
	@echo "  build            - Build the exporter"
	@echo "  test             - Run tests"
	@echo "  test-coverage    - Run tests with coverage report"
	@echo "  lint             - Run linter"
	@echo "  fmt              - Format code"
	@echo "  tidy             - Tidy go modules"
	@echo "  install-tools    - Install development tools"
	@echo "  clean            - Clean build artifacts"
	@echo "  build-collector  - Build custom OTel Collector with Arc exporter"
	@echo "  run-example      - Run Docker Compose example"
	@echo "  stop-example     - Stop Docker Compose example"
	@echo "  gosum            - Download dependencies"

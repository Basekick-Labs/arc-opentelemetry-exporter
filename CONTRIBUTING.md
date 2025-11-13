# Contributing to Arc OpenTelemetry Exporter

Thank you for your interest in contributing! This document provides guidelines for contributing to the Arc OpenTelemetry Exporter.

## Development Setup

### Prerequisites

- Go 1.22 or later
- Make
- Docker and Docker Compose (for testing)

### Getting Started

1. **Clone the repository**

```bash
git clone https://github.com/basekick-labs/arc-opentelemetry-exporter.git
cd arc-opentelemetry-exporter
```

2. **Install dependencies**

```bash
make gosum
```

3. **Install development tools**

```bash
make install-tools
```

4. **Run tests**

```bash
make test
```

## Development Workflow

### Building

```bash
# Build the exporter
make build

# Build a custom OTel Collector with the Arc exporter
make build-collector
```

### Testing

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run linter
make lint

# Format code
make fmt
```

### Running Locally

```bash
# Start Arc and OTel Collector with Docker Compose
make run-example

# Stop the example
make stop-example
```

## Code Structure

```
arc-opentelemetry-exporter/
├── config.go              # Exporter configuration
├── factory.go             # Factory for creating exporter instances
├── traces_exporter.go     # Traces exporter implementation
├── metrics_exporter.go    # Metrics exporter implementation
├── logs_exporter.go       # Logs exporter implementation
├── examples/              # Example configurations
│   ├── otel-collector-config.yaml
│   ├── docker-compose.yaml
│   └── builder-config.yaml
├── go.mod                 # Go module definition
└── README.md              # Documentation
```

## Adding Features

### Adding New Configuration Options

1. Update `config.go` to add the new field
2. Update `Validate()` method if needed
3. Update the factory default config in `factory.go`
4. Update example configurations in `examples/`
5. Update README.md documentation

### Modifying Data Format

If you need to change how data is transformed:

1. Update the relevant exporter file (`traces_exporter.go`, `metrics_exporter.go`, or `logs_exporter.go`)
2. Ensure the columnar format matches Arc's expectations
3. Add tests to verify the transformation
4. Update documentation

## Testing

### Unit Tests

Write unit tests for any new functionality. Tests should:

- Be placed in `*_test.go` files
- Use table-driven tests where appropriate
- Mock external dependencies
- Achieve good code coverage

Example:

```go
func TestTracesExporter_tracesToColumnar(t *testing.T) {
    tests := []struct {
        name    string
        input   ptrace.Traces
        want    []byte
        wantErr bool
    }{
        // Test cases
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Integration Tests

Integration tests require:

1. Arc instance running (use Docker)
2. Test data generation
3. Verification of data in Arc

## Code Style

### Go Conventions

- Follow standard Go formatting (`gofmt`)
- Use meaningful variable names
- Add comments for exported functions
- Keep functions focused and small

### Error Handling

Always provide context in errors:

```go
if err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}
```

### Logging

Use structured logging with zap:

```go
e.logger.Debug("Operation completed",
    zap.String("operation", "export"),
    zap.Int("count", count))
```

## Pull Request Process

1. **Fork the repository**
2. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```
3. **Make your changes**
4. **Run tests and linting**
   ```bash
   make lint test
   ```
5. **Commit your changes**
   ```bash
   git commit -m "feat: add new feature"
   ```
6. **Push to your fork**
   ```bash
   git push origin feature/your-feature-name
   ```
7. **Open a Pull Request**

### Commit Message Format

Follow conventional commits:

- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation changes
- `test:` - Adding or updating tests
- `refactor:` - Code refactoring
- `perf:` - Performance improvements
- `chore:` - Maintenance tasks

## Releasing

Releases are managed by maintainers. The process:

1. Update version in relevant files
2. Update CHANGELOG.md
3. Create a git tag: `git tag v0.x.0`
4. Push the tag: `git push origin v0.x.0`
5. GitHub Actions will build and publish the release

## Getting Help

- Open an issue for bug reports or feature requests
- Join our [Discord community](https://discord.gg/nxnWfUxsdm)
- Check the [Arc documentation](https://basekick.net/docs)

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License.

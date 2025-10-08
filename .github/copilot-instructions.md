# GitHub Copilot Instructions for tcp-sproxy

## Project Overview

tcp-sproxy is a minimal TCP stream proxy that tunnels arbitrary TCP connections over NATS messaging system. It's an experimental project suitable for demos and experiments.

**Architecture:**
- **Server**: Accepts proxy requests via NATS and connects to target TCP services
- **Client**: Listens on a local TCP port and forwards connections through NATS to the server
- **Transport**: Uses NATS as the messaging layer for proxying TCP traffic

**Key Components:**
- `cmd/server`: NATS proxy server entry point
- `cmd/client`: NATS proxy client entry point  
- `internal/domain`: Domain models, entities, and services (business logic)
- `internal/application`: Use cases and application layer
- `internal/infrastructure`: Transport (NATS) and repositories (in-memory)
- `integration_tests`: Docker Compose setup with Redis-based end-to-end tests

## Technology Stack

- **Language**: Go 1.24+
- **Messaging**: NATS (nats.io/nats.go v1.44.0)
- **CLI Framework**: Cobra v1.9.1 with Viper v1.20.1 for configuration
- **Logging**: Logrus v1.9.3
- **Testing**: stretchr/testify v1.10.0
- **Integration Testing**: Docker Compose with Redis and NATS services

## Coding Standards

### General Go Practices
- Follow standard Go conventions and idioms
- Use `gofmt` for formatting (enforced in CI)
- Run `go vet` to catch common mistakes
- Write clear, concise comments for exported functions and types
- Keep functions small and focused on a single responsibility

### Architecture Patterns
- Follow **Clean Architecture** principles:
  - Domain layer is independent of infrastructure
  - Use cases orchestrate domain operations
  - Infrastructure depends on domain, not vice versa
- Use dependency injection via constructors
- Keep interfaces small and focused

### Error Handling
- Always check and handle errors explicitly
- Wrap errors with context using `fmt.Errorf("context: %w", err)`
- Log errors with appropriate levels (error, warn, info, debug)
- Use structured logging with logrus fields

### Naming Conventions
- Use meaningful variable and function names
- Interface names end with `-er` suffix when appropriate (e.g., `Transport`, `Repository`)
- Test files use `_test.go` suffix
- Mock implementations go in `internal/test/mocks/`

## Configuration

Both binaries use **Cobra + Viper** for configuration:

### Configuration Sources (priority order)
1. Command-line flags (highest priority)
2. Environment variables
3. Configuration files (`.nats-proxy-server.*` or `.nats-proxy-client.*`)
4. Default values (lowest priority)

### Server Configuration
- Flags: `--nats-url`, `--listen-addr`, `--log-level`, `--config`
- Env vars: `NATS_URL`, `LOG_LEVEL`
- Log levels: debug, info, warn, error

### Client Configuration  
- Flags: `--nats-url`, `--listen-addr`, `--remote-addr`, `--proxy-addr`, `--log-level`
- Env vars: `NATS_URL`, `LISTEN_ADDR`, `REMOTE_ADDR`, `PROXY_ADDR`, `LOG_LEVEL`

## Testing Requirements

### Unit Tests
- Write unit tests for all business logic in domain and application layers
- Use testify for assertions and mocking
- Run tests with race detector: `go test -race ./...`
- Aim for meaningful test coverage (current CI generates coverage reports)
- Mock external dependencies (NATS transport, repositories)

### Integration Tests
- Integration tests use Docker Compose with real Redis and NATS services
- Test file: `integration_tests/docker-compose.yml`
- Run with: `make test-integration`
- Tests verify end-to-end proxy functionality with Redis

### Test Commands
```bash
# Unit tests
go test ./...

# With race detection
go test -race ./...

# With coverage
go test -race -coverprofile=coverage.out -covermode=atomic ./...

# Integration tests
make test-integration
```

## Building and Running

### Local Development
```bash
# Build binaries
go build -o bin/nats-proxy-server ./cmd/server
go build -o bin/nats-proxy-client ./cmd/client

# Run directly
go run ./cmd/server --help
go run ./cmd/client --help
```

### Docker
```bash
# Server image (root Dockerfile)
docker build -t tcp-sproxy-server .

# Dev/integration image (contains both server and client)
docker build -t tcp-sproxy-bundle -f integration_tests/build/Dockerfile .
```

### Dependencies
```bash
# Download dependencies
go mod download

# Verify dependencies
go mod verify

# Update dependencies
go get -u ./...
go mod tidy
```

## CI/CD Workflows

### Unit Tests (`.github/workflows/test.yml`)
- **Triggers**: Pull requests, pushes to main/master
- **Actions**: 
  - Run `go vet`
  - Execute tests with race detection
  - Generate coverage reports
  - Upload to Codecov

### Integration Tests (`.github/workflows/integration-test.yml`)
- **Triggers**: Manual dispatch, daily at 2 AM UTC, pushes to main/master
- **Services**: Redis and NATS containers
- **Actions**: Run `make test-integration`

## Protocol Details

### Control Plane
- Client requests proxy via NATS subject: `proxy.request`
- Request includes: `remote_host` and `remote_port`
- Server responds with generated `connection_id`

### Data Plane
- Client → Server: `p.data.to_server.{connectionID}`
- Server → Client: `p.data.to_client.{connectionID}`
- Messages serialized as JSON using `transport.Message` structure

## Important Notes

### Security & Limitations
- **No authentication, rate limiting, or encryption** - this is experimental
- Use NATS security features and network controls as needed
- `--proxy-addr` on client is currently informational/reserved
- Server does not expose HTTP endpoint (despite Dockerfile healthcheck reference)

### When Making Changes

1. **Domain changes**: Keep domain logic pure and independent of infrastructure
2. **Transport changes**: Ensure backward compatibility with existing message formats
3. **Configuration changes**: Update both code and README.md documentation
4. **Breaking changes**: Clearly document in commit messages
5. **New dependencies**: Minimize external dependencies, justify additions
6. **CI/CD changes**: Test workflow changes in a branch first

## Common Development Tasks

### Adding a New Feature
1. Start with domain model changes if needed
2. Update or create use cases in application layer
3. Implement infrastructure support (transport, repository)
4. Add unit tests for all layers
5. Update integration tests if end-to-end behavior changes
6. Update README.md with new configuration options or usage

### Fixing Bugs
1. Write a failing test that reproduces the bug
2. Fix the bug with minimal changes
3. Ensure all tests pass
4. Update documentation if behavior changes

### Refactoring
1. Ensure all tests pass before starting
2. Make incremental changes
3. Run tests frequently during refactoring
4. Keep test coverage at same or better level

## Code Review Checklist

- [ ] Code follows Go conventions and project architecture
- [ ] All exported functions and types have comments
- [ ] Error handling is comprehensive
- [ ] Tests are added/updated as needed
- [ ] No new linter warnings or errors
- [ ] Documentation is updated if needed
- [ ] CI/CD workflows pass
- [ ] Changes are minimal and focused

## Getting Help

- Review README.md for project setup and usage
- Check existing tests for examples of patterns
- Look at internal/test/mocks for mocking examples
- Review issues/ directory for known issues and solutions

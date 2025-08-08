# tcp-sproxy (NATS TCP Stream Proxy)

A minimal TCP stream proxy that tunnels arbitrary TCP connections over NATS. It consists of:
- A server that accepts proxy requests via NATS and connects to the target TCP service.
- A client that listens on a local TCP port and forwards each incoming connection through NATS to the server, which then talks to the target.

This is useful when direct TCP connectivity to a service is not possible, but NATS connectivity is available.

## Status
Experimental. Suitable for demos and experiments. Security, auth, and advanced features are out of scope for now.

## How it works (high level)
- Control plane:
  - Client requests a new proxy connection by sending a NATS request to subject `proxy.request` with metadata: `remote_host` and `remote_port`.
  - Server replies with a generated `connection ID`.
- Data plane:
  - Client -> Server bytes are published to `p.data.to_server.{connectionID}`.
  - Server -> Client bytes are published to `p.data.to_client.{connectionID}`.
- The server maintains a TCP connection to the remote service and relays bytes between it and the client over NATS.

Internally, messages are serialized as JSON using a simple `transport.Message` structure.

## Repository layout
- cmd/server: NATS proxy server
- cmd/client: NATS proxy client
- internal/...: domain, use cases, NATS transport, and in-memory repository
- integration_tests: docker-compose setup and a Redis-based end‑to‑end test

## Requirements
- Go 1.24+
- A running NATS server (e.g., `nats:2.9-alpine`)
- Docker and Docker Compose (for integration tests or containerized runs)

## Build
Build both binaries locally:

```bash
go build -o bin/nats-proxy-server ./cmd/server
go build -o bin/nats-proxy-client ./cmd/client
```

Or run directly:

```bash
go run ./cmd/server --help
go run ./cmd/client --help
```

## Quick start locally (Redis example)
1) Start NATS:

```bash
docker run --rm -p 4222:4222 --name nats nats:2.9-alpine
```

2) Start Redis locally for the demo:

```bash
docker run --rm -p 6379:6379 --name redis redis:7-alpine
```

3) Start the proxy server (in another terminal):

```bash
bin/nats-proxy-server --nats-url nats://127.0.0.1:4222 --log-level info
```

4) Start the proxy client to expose a local port that forwards to Redis via NATS:

```bash
bin/nats-proxy-client \
  --nats-url nats://127.0.0.1:4222 \
  --listen-addr 127.0.0.1:6380 \
  --remote-addr 127.0.0.1:6379 \
  --proxy-addr localhost:8081  # currently informational/reserved
```

5) Test with redis-cli:

```bash
redis-cli -h 127.0.0.1 -p 6380 PING
redis-cli -h 127.0.0.1 -p 6380 SET key value
redis-cli -h 127.0.0.1 -p 6380 GET key
```

If everything is wired, you should see PONG and value replies proxied over NATS.

## Configuration
Both binaries use Cobra + Viper. You can configure via flags, env vars, or config files.

### Server (nats-proxy-server)
- Flags:
  - `--nats-url` (default `nats://localhost:4222`)
  - `--listen-addr` (default `:8080`) — currently not used by the server
  - `--log-level` (debug|info|warn|error; default `info`)
  - `--config` (path to config file; default search: `./` then `$HOME`, file name `.nats-proxy-server.*`)
- Environment:
  - `NATS_URL` maps to `nats.url`
  - `LOG_LEVEL` maps to `log.level`
- Example YAML (e.g., `.nats-proxy-server.yaml`):

```yaml
nats:
  url: nats://localhost:4222
log:
  level: info
```

### Client (nats-proxy-client)
- Flags:
  - `--nats-url` (default `nats://localhost:4222`)
  - `--listen-addr` (default `0.0.0.0:8082`) — local TCP listen address
  - `--remote-addr` (default `redis:6379`) — target service address the server will dial
  - `--proxy-addr` (default `proxy-server:8081`) — currently informational/reserved in NATS transport
  - `--log-level` (debug|info|warn|error; default `info`)
  - `--config` (path to config file; default search: `./` then `$HOME`, file name `.nats-proxy-client.*`)
- Environment:
  - `NATS_URL` → `nats.url`
  - `LISTEN_ADDR` → `client.listen_addr`
  - `REMOTE_ADDR` → `client.remote_addr`
  - `PROXY_ADDR` → `client.proxy_addr`
  - `LOG_LEVEL` → `log.level`
- Example YAML (e.g., `.nats-proxy-client.yaml`):

```yaml
nats:
  url: nats://localhost:4222
client:
  listen_addr: 0.0.0.0:8082
  remote_addr: redis:6379
  proxy_addr: proxy-server:8081
log:
  level: info
```

## Docker
- Server-only image (root Dockerfile):

```bash
docker build -t tcp-sproxy-server -f Dockerfile .
# Then run with a NATS_URL env variable
# NOTE: The Dockerfile exposes 8080 and defines a healthcheck path; the server does not expose an HTTP endpoint.
docker run --rm --network host -e NATS_URL=nats://127.0.0.1:4222 tcp-sproxy-server
```

- Dev/integration image (contains both server and client):

```bash
docker build -t tcp-sproxy-bundle -f integration_tests/build/Dockerfile .
```

## Tests
- Unit tests:

```bash
go test ./...
```

- Integration test (Redis through the proxy using Docker Compose):

```bash
# Using Makefile
make test-integration

# Or manually
docker-compose -f integration_tests/docker-compose.yml up -d --build
# the test-runner service will execute integration tests
# when finished
docker-compose -f integration_tests/docker-compose.yml down
```

## Logging
Both components use logrus. Levels: `debug`, `info`, `warn`, `error` (set via `--log-level` or `LOG_LEVEL`).

## Notes & Limitations
- No authentication, rate limiting, or encryption provided by this project. Use NATS security features and network controls as appropriate.
- The `--proxy-addr` on the client is currently reserved/informational in the NATS transport implementation.
- The server does not currently expose an HTTP endpoint (despite the Dockerfile healthcheck example).

## License
No license specified in this repository at the moment.

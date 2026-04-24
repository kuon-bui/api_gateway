# API Gateway (Go + Gin)

API Gateway production-ready built with Go and Gin.

## Goals

- Centralized entry point for backend services.
- Config-driven static routing via YAML file.
- JWT validation at gateway layer.
- Rate limiting by IP and API key.
- Structured logs, metrics, and tracing hooks.
- Clean architecture and strict coding standards.

## Tech Stack

- Go (latest stable)
- Gin (`github.com/gin-gonic/gin`)
- YAML config (`gopkg.in/yaml.v3`)
- Rate limiting (`golang.org/x/time/rate`)

## Core Principles

- 12-factor app conventions.
- Stateless runtime.
- Configuration through environment and config files.
- Fail-fast startup when config is invalid.
- Mandatory quality gates in CI.

## Initial Project Layout

```text
api-gateway/
  cmd/gateway/
  internal/
    app/
    config/
    domain/
    middleware/
    proxy/
    transport/
  configs/
    gateway.yaml
  docs/
  Makefile
  .golangci.yml
```

## Documentation Index

- [Architecture](docs/01-architecture.md)
- [Coding Standards](docs/02-coding-standards.md)
- [Implementation Plan](docs/03-implementation-plan.md)
- [Definition of Done](docs/04-definition-of-done.md)
- [Routing Config Spec](docs/05-routing-config-spec.md)

## Next Step

Bootstrap implementation is complete (module init, app skeleton, config loader, router/proxy flow, JWT, rate limiting, metrics endpoint).

Current priorities:

- Add OpenTelemetry tracing spans for inbound/outbound requests.
- Add integration tests for proxy/auth/rate-limit behavior.
- Wire CI workflow for lint/test/race/build quality gates.

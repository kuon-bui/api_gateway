# Architecture - API Gateway (Go + Gin)

## 1. Scope

Gateway responsibilities:

- Receive external traffic.
- Validate authentication token (JWT).
- Apply rate limiting (IP/API key).
- Route request to upstream service based on static route map.
- Return normalized gateway errors.
- Emit observability telemetry (logs/metrics/traces).

Out of scope in phase 1:

- Dynamic service discovery.
- Distributed circuit breaker.
- Advanced payload transformation.

## 2. Logical Layers (Clean Architecture)

- `transport`: HTTP handlers and server lifecycle.
- `middleware`: cross-cutting concerns (auth, rate limit, request id, logging).
- `usecase` (inside `app`): request pipeline orchestration and route decision.
- `domain`: models and contracts.
- `infrastructure`: config source, proxy client, telemetry adapters.

## 3. Request Flow

1. Incoming request enters Gin engine.
2. Middleware chain runs:
   - request id
   - access log context
   - JWT validation
   - rate limiting
3. Route resolver maps path/method to upstream target.
4. Reverse proxy forwards request to upstream with timeout/cancellation propagation.
5. Response is streamed back; gateway logs metrics and status.

## 4. Runtime Components

- HTTP server (Gin).
- Config loader (YAML + env overrides).
- Route resolver (prefix/static map).
- Proxy adapter (`net/http/httputil`-based).
- Auth validator (JWT verifier).
- Rate limiter store.
- Observability adapter.

## 5. Reliability Defaults

- Startup fail-fast on invalid config.
- Global server read/write/idle timeouts.
- Upstream request timeout.
- Graceful shutdown with drain period.

## 6. Security Defaults

- Reject malformed/expired JWT.
- Optional issuer/audience enforcement.
- Header allow-list when forwarding claims.
- Limit exposed internal error details.

## 7. Deploy Model

- Horizontal scaling via stateless instances.
- Config mounted by file and environment.
- Logs to stdout.
- Health endpoints:
  - `/healthz`: process liveness.
  - `/readyz`: dependency readiness.

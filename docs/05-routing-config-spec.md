# Routing Config Specification (v1)

## 1. File Format

- YAML file loaded at startup.
- Default location: `configs/gateway.yaml`.
- Environment variable can override path.

## 2. Schema

```yaml
server:
  port: 8080
  read_timeout_ms: 5000
  write_timeout_ms: 0
  idle_timeout_ms: 60000
  tls:
    enabled: false
    cert_file: ""
    key_file: ""

proxy:
  timeout_ms: 0

telemetry:
  enabled: false
  service_name: "api-gateway"
  otlp_endpoint: "localhost:4317"
  insecure: true

security:
  jwt:
    enabled: true
    issuer: "https://issuer.example.com"
    audience: "gateway-clients"

admin:
  enabled: false
  api_key: ""

rate_limit:
  enabled: true
  rps: 20
  burst: 40
  by_api_key_header: "X-API-Key"
  backend: "memory"
  redis_address: "localhost:6379"
  redis_password: ""
  redis_db: 0
  redis_key_prefix: "gateway:rl"

routes:
  - name: users
    methods: ["GET", "POST"]
    path_prefix: "/users"
    upstream: "http://user-service:9001"
    forward_headers: ["X-Request-ID", "X-Correlation-ID"]
    circuit_breaker:
      enabled: true
      failure_threshold: 5
      open_timeout_ms: 5000
      half_open_max_requests: 2
    retry:
      enabled: true
      max_attempts: 3
      backoff_ms: 100
    rate_limit:
      enabled: true
      rps: 10
      burst: 20

  - name: orders
    methods: ["GET"]
    path_prefix: "/orders"
    # Multiple upstreams with load balancing + passive health checks.
    load_balancing: "weighted"   # round_robin (default) | weighted | random
    upstreams:
      - url: "http://order-service-a:9002"
        weight: 3
      - url: "http://order-service-b:9002"
        weight: 1
    health_check:
      passive:
        enabled: true
        failure_threshold: 3
        cooldown_ms: 10000
    rate_limit:
      enabled: false
```

### Upstreams

A route targets backends via **either** of two mutually exclusive fields:

- `upstream` (string) — a single backend URL. Backward compatible.
- `upstreams` (list) — a pool of `{ url, weight }` entries for load balancing.

`load_balancing` selects the strategy across the pool:

- `round_robin` (default) — cycle through upstreams in order.
- `weighted` — smooth weighted round-robin honouring each `weight`.
- `random` — uniformly random healthy upstream.

`health_check.passive` ejects an upstream after `failure_threshold` consecutive
failures (transport error or `502`/`503`/`504`) and re-admits it (half-open)
after `cooldown_ms`. When `retry` is enabled, retry attempts fail over to other
healthy upstreams; WebSocket connections pick a single upstream (no failover).
When every upstream is ejected, the gateway returns `503 NO_HEALTHY_UPSTREAM`.

## 3. Validation Rules

- `routes` must not be empty.
- Route names must be unique.
- `path_prefix` must start with `/`.
- Exactly one of `upstream` (single) or `upstreams` (pool) must be set per route.
- `upstream` / each `upstreams[*].url` must be a valid HTTP/HTTPS URL.
- `upstreams[*].weight` must not be negative; when `load_balancing=weighted` it must be positive.
- `load_balancing`, when set, must be one of `round_robin`, `weighted`, `random`.
- If `health_check.passive.enabled=true`, `failure_threshold` and `cooldown_ms` must be positive.
- `methods` must contain valid HTTP methods.
- `read_timeout_ms`, `idle_timeout_ms`, and rate values must be positive.
- `write_timeout_ms` must be zero or positive (`0` disables write timeout for long-lived streams like SSE).
- `proxy.timeout_ms` must be zero or positive (`0` disables upstream connect timeout).
- If `server.tls.enabled=true`, both `server.tls.cert_file` and `server.tls.key_file` are required.
- If `admin.enabled=true`, `admin.api_key` is required.
- If `telemetry.enabled=true`, `telemetry.otlp_endpoint` is required.
- `forward_headers` is optional. If configured, only listed headers are forwarded.
- If `rate_limit.backend=redis`, `rate_limit.redis_address` is required.
- `routes[*].rate_limit` is optional.
- If `routes[*].rate_limit.enabled=true`, both `routes[*].rate_limit.rps` and `routes[*].rate_limit.burst` must be positive.
- `routes[*].circuit_breaker` is optional.
- If `routes[*].circuit_breaker.enabled=true`, `failure_threshold`, `open_timeout_ms`, and `half_open_max_requests` must be positive.
- `routes[*].retry` is optional.
- If `routes[*].retry.enabled=true`, `max_attempts` must be greater than 1 and `backoff_ms` must be zero or positive.
- Route prefixes cannot conflict with reserved system endpoints `/healthz`, `/readyz`, `/metrics`.

## 4. Resolution Strategy

- Match by HTTP method first.
- Then match longest `path_prefix`.
- Within a matched route, an upstream is chosen per attempt by the `load_balancing` strategy, skipping passively-ejected upstreams.
- Retry is applied only for idempotent requests and only on upstream errors / 502 / 503 / 504; each retry selects another upstream (failover).
- Rate limiting precedence: route rate limit override (if configured) takes priority over global `rate_limit`.
- If no route matches, return `404` gateway route-not-found response.

## 5. Error Contract (proposed)

```json
{
  "error": {
    "code": "ROUTE_NOT_FOUND",
    "message": "No upstream route matched"
  },
  "request_id": "..."
}
```

## 6. Compatibility Notes

- Backward-compatible fields may be added in minor versions.
- Breaking schema changes require explicit version bump and migration guide.

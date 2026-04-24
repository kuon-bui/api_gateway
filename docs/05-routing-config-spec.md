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
  write_timeout_ms: 10000
  idle_timeout_ms: 60000

proxy:
  timeout_ms: 15000

security:
  jwt:
    enabled: true
    issuer: "https://issuer.example.com"
    audience: "gateway-clients"

rate_limit:
  enabled: true
  rps: 20
  burst: 40
  by_api_key_header: "X-API-Key"

routes:
  - name: users
    methods: ["GET", "POST"]
    path_prefix: "/users"
    upstream: "http://user-service:9001"

  - name: orders
    methods: ["GET"]
    path_prefix: "/orders"
    upstream: "http://order-service:9002"
```

## 3. Validation Rules

- `routes` must not be empty.
- Route names must be unique.
- `path_prefix` must start with `/`.
- `upstream` must be valid HTTP/HTTPS URL.
- `methods` must contain valid HTTP methods.
- Timeout and rate values must be positive.

## 4. Resolution Strategy

- Match by HTTP method first.
- Then match longest `path_prefix`.
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

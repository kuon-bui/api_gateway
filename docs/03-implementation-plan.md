# Implementation Plan - Production-Ready API Gateway

## Phase 1: Bootstrap Foundation

Deliverables:

- Go module initialization.
- Project structure (cmd/internal/configs/docs).
- Base app entrypoint and HTTP server skeleton.
- Lint config and Makefile tasks.

Acceptance:

- `make lint` and `make test` runnable.
- Binary starts and exposes `/healthz`.

## Phase 2: Config-Driven Routing

Deliverables:

- YAML route config schema.
- Config loader + validation.
- Route resolver by method/path prefix.

Acceptance:

- Startup fails on invalid config.
- Valid config maps traffic to expected route target.

## Phase 3: Reverse Proxy Core

Deliverables:

- Upstream forwarding with timeout propagation.
- Standardized gateway error responses.
- Header forwarding policy.

Acceptance:

- Requests and responses pass-through correctly.
- Timeout returns normalized gateway timeout error.

## Phase 4: Security and Rate Limiting

Deliverables:

- JWT validation middleware.
- Rate limiting middleware (IP/API key).
- Claim forwarding allow-list.

Acceptance:

- Invalid token is blocked.
- Exceeded quota returns consistent 429 response.

## Phase 5: Observability and Hardening

Deliverables:

- Structured request logs.
- Metrics endpoint and core counters/histograms.
- Graceful shutdown and readiness checks.

Acceptance:

- Logs include mandatory fields.
- Metrics visible and updated under load.

## Phase 6: CI/CD Quality Gates

Deliverables:

- CI pipeline for fmt/lint/test/race.
- Basic vulnerability scanning.
- Release artifact packaging.

Acceptance:

- PR cannot merge when quality gates fail.

## Risks and Mitigations

- Risk: Config drift between environments.
  - Mitigation: schema validation + startup fail-fast.
- Risk: Limiter inconsistency in multi-instance mode.
  - Mitigation: define limiter abstraction for Redis upgrade path.
- Risk: JWT key rotation downtime.
  - Mitigation: support JWKS cache and background refresh.

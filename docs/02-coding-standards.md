# Coding Standards - Go API Gateway

## 1. Language and Formatting

- Use stable Go version defined in `go.mod`.
- Mandatory formatting: `gofmt` + `goimports`.
- CI must fail if formatting is incorrect.

## 2. Lint Policy

Use `golangci-lint` in strict mode with at least:

- `govet`
- `staticcheck`
- `errcheck`
- `ineffassign`
- `unused`
- `gosimple`
- `gocritic`
- `revive`

Rules:

- No ignored errors unless explicitly justified.
- Keep cyclomatic complexity bounded.
- Avoid unnecessary global state.

## 3. Project Conventions

- `cmd/`: application entrypoints only.
- `internal/`: business and infrastructure logic.
- Keep packages focused and cohesive.
- Export only what other packages need.

## 4. Error Handling

- Return wrapped errors with context (`fmt.Errorf("...: %w", err)`).
- Map internal errors to standardized HTTP errors.
- Never expose sensitive internals to clients.

## 5. Logging

- Structured logs only.
- Required fields for request logs:
  - `request_id`
  - `method`
  - `path`
  - `status`
  - `latency_ms`
  - `upstream`

## 6. Security Rules

- Validate all external inputs.
- Never trust forwarded headers by default.
- Avoid storing secrets in source files.
- Read secrets from environment or secret manager.

## 7. Testing

- Unit tests for parser, resolver, middleware logic.
- Integration tests for proxy flow with mock upstreams.
- Run tests with race detector in CI (`go test -race`).

## 8. Git and Review

- Small, focused commits.
- Pull request must include:
  - rationale
  - risk assessment
  - test evidence
- No merge when lint/test gates fail.

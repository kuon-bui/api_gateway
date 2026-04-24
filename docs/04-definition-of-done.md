# Definition of Done (DoD)

A feature is considered done when all conditions below are satisfied.

## Functional

- Requirement behavior is implemented and verified.
- Edge cases are handled and documented.
- Error responses are consistent with gateway contract.

## Code Quality

- Code is formatted (`gofmt`, `goimports`).
- Lint passes with strict profile.
- No unresolved TODOs without linked follow-up task.

## Testing

- Unit tests added or updated.
- Integration tests added for external behavior changes.
- Race test passes where applicable.

## Security

- Input validation is present.
- Sensitive data is not leaked in logs/errors.
- Auth and authorization impact is reviewed.

## Operations

- Observability hooks added (logs/metrics/traces) where needed.
- Health/readiness behavior updated if relevant.
- Backward compatibility impact is assessed.

## Documentation

- Relevant docs updated in `docs/`.
- Config changes include examples.
- Breaking changes are clearly noted.

# Testing

## Scope
Every package under `internal/` must have a corresponding `_test.go` file.
Tests are not optional — untested packages are treated as incomplete.

## Package Coverage Requirements

| Package | What to test |
|---|---|
| `internal/domain` | Aggregate invariants, state machine transitions, domain event construction, business rules (index eligibility, stock status transitions) |
| `internal/app` | Use-case orchestration: happy path, invariant violations, optimistic lock conflicts, idempotency key hits |
| `internal/repository` | SQL queries, cache read/write/invalidation, Bloom filter load and refresh, OpenSearch index operations |

## Test Types

### Unit tests (`_test.go` alongside the package)
- No external dependencies — use interfaces and fakes/stubs defined in `internal/domain`
- Cover all exported functions and methods
- Table-driven tests (`[]struct{ name, input, want }`) for any function with multiple input cases
- File naming: `<file>_test.go` mirrors the file under test

### Integration tests (`internal/<pkg>/integration_test.go`)
- Require real infrastructure (Postgres, Valkey, OpenSearch)
- Guarded by a build tag: `//go:build integration`
- Run via `make test-integration` which spins up `docker compose` before executing
- Never run as part of the default `make test` target

## Conventions
- Use the standard library `testing` package — no third-party test frameworks
- Assertions via `if got != want { t.Errorf(...) }` or a minimal helper; no `testify` unless the team explicitly adopts it
- Fakes (in-memory implementations of domain interfaces) live in `internal/domain/testutil/` and are shared across packages
- Test data builders (constructors that return valid domain objects with sensible defaults) live alongside the fakes in `internal/domain/testutil/`
- No `init()` in test files
- Tests must be deterministic — no `time.Now()` or `rand` without injection

## Running Tests
```
make test                  # unit tests only (no build tags)
make test-integration      # integration tests (requires Docker)
make test-cover            # unit tests with coverage report
```

Coverage target: **80% line coverage** per package minimum, enforced in CI.

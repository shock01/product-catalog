# Go Linting Standards

## Tool
All linting is done via [`golangci-lint`](https://golangci-lint.run/) — a single binary that runs multiple linters in one pass.

- Config: `.golangci.yml` at the repo root
- Run: `make lint`
- CI must fail on any lint error — no warnings-only mode

Install:
```
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
```

Add to `make tools` so `make lint` has no hidden prerequisite.

---

## Essential Linters

### Correctness
| Linter | Why |
|---|---|
| `govet` | Reports constructs the Go compiler accepts but are almost certainly bugs (e.g. `printf` format/arg mismatches, unreachable code after `return`) |
| `staticcheck` | Deep static analysis: deprecated API usage, impossible conditions, redundant code, misuse of `sync` primitives |
| `errcheck` | Every returned `error` must be checked or explicitly discarded with `_` — unchecked errors are silent failures |
| `ineffassign` | Catches assignments whose value is never read — usually a logic error or leftover from a refactor |
| `unused` (via `staticcheck`) | Exported identifiers that are never used outside the package they are declared in |

### Security
| Linter | Why |
|---|---|
| `gosec` | Scans for common security issues: hardcoded credentials, weak crypto, SQL injection via string concat, unsafe use of `os/exec`, `G104` (unchecked errors in defer) |

### HTTP / Network
| Linter | Why |
|---|---|
| `bodyclose` | HTTP response bodies must be closed — forgetting leaks a connection from the pool |
| `noctx` | HTTP requests must carry a `context.Context` — bare `http.Get` etc. are not cancellable |

### Error handling
| Linter | Why |
|---|---|
| `wrapcheck` | Errors returned from external packages must be wrapped with `%w` so callers can use `errors.Is` / `errors.As` |

### Code quality
| Linter | Why |
|---|---|
| `revive` | Opinionated style linter (replaces `golint`): exported symbols need doc comments, stuttering names, unused parameters |
| `gocritic` | Large collection of micro-optimisations and style checks not covered by other linters |
| `exhaustive` | `switch` statements on enum types must handle every case (or have an explicit `default`) — prevents silent gaps when a new enum value is added |
| `cyclop` | Cyclomatic complexity per function — default max 10; functions above this threshold must be refactored |
| `gocognit` | Cognitive complexity (how hard code is to read) — stricter than cyclomatic; default max 15 |

---

## Exclusions
- `internal/gen/` is always excluded — it is generated code
- `cmd/*/main.go` is excluded from `wrapcheck` (top-level error handling is intentionally different)
- Test files (`*_test.go`) are excluded from `exhaustive` and `gocognit`

---

## Configuration
The full linter config lives in `.golangci.yml`. Key settings:

```yaml
linters:
  enable:
    - govet
    - staticcheck
    - errcheck
    - ineffassign
    - gosec
    - bodyclose
    - noctx
    - wrapcheck
    - revive
    - gocritic
    - exhaustive
    - cyclop
    - gocognit

issues:
  exclude-dirs:
    - internal/gen
```

---

## Makefile
`make lint` runs golangci-lint. `make tools` installs it.
`make lint` is a required step in CI before `make build`.

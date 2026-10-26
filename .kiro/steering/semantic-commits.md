# Semantic Commits

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification.

## Format

```
<type>(<scope>): <short summary>

[optional body]

[optional footer(s)]
```

- Subject line: imperative mood, lowercase, no trailing period, max 72 chars
- Body: wrap at 72 chars, explain *why* not *what*
- Footer: `BREAKING CHANGE:` or issue refs (`Closes #123`)

---

## Types

| Type | When to use |
|---|---|
| `feat` | New behaviour visible to a consumer of the service (new endpoint, new field, new event) |
| `fix` | Corrects a bug in existing behaviour |
| `refactor` | Code change that neither adds a feature nor fixes a bug |
| `perf` | Performance improvement with no behaviour change |
| `test` | Adding or correcting tests only |
| `docs` | Documentation, ADRs, steering files, README |
| `chore` | Tooling, dependencies, CI config, Makefile, `.gitignore` |
| `build` | Changes to the build system or generated code (`make generate`, Dockerfiles) |
| `ci` | CI/CD pipeline changes |

---

## Scopes

Scope is optional but recommended. Use the layer or subsystem being changed:

| Scope | Covers |
|---|---|
| `domain` | `internal/domain/` |
| `usecase` | `internal/usecase/` |
| `handler` | `internal/handler/` |
| `repository` | `internal/repository/` |
| `gen` | `internal/gen/` — generated code |
| `api` | `api/openapi.yaml` contract changes |
| `migration` | `migrations/` |
| `indexer` | `cmd/indexer/` |
| `importer` | `cmd/importer/` |
| `infra` | `compose.yaml`, `compose/`, infrastructure config |
| `steering` | `.kiro/steering/` |

---

## Examples

```
feat(api): add stock_status filter to search endpoint

fix(domain): validate that published products have at least one enabled channel

refactor(usecase): wrap repository errors with fmt.Errorf context

perf(repository): add composite index on (status, updated_at) for list query

test(domain): add table-driven tests for Product.Validate invariants

docs(adr): add ADR-011 for bloom filter refresh strategy

chore: add golangci-lint to make tools target

build(gen): regenerate api.gen.go from updated openapi spec

feat(api)!: rename slug field to url_slug in ProductTranslation

BREAKING CHANGE: clients must update to the new field name
```

---

## Breaking changes

- Append `!` after the type/scope: `feat(api)!:`
- Always include a `BREAKING CHANGE:` footer describing the impact
- Breaking changes to the OpenAPI contract require a version bump — see `url-structure.md`

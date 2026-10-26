# Language & Project Layout

## Language
- Go (Golang) — all service code is written in Go

## Package Layout
Follow the [golang-standards/project-layout](https://github.com/golang-standards/project-layout):

```
product-catalog/
├── cmd/
│   ├── api/          # HTTP API server entrypoint
│   └── indexer/      # OpenSearch index job entrypoint
├── internal/
│   ├── domain/       # Entities, value objects, domain events, business rules
│   ├── repository/   # Database + cache adapters (Postgres, Valkey, OpenSearch)
│   └── app/          # Application services / use-cases
├── api/              # OpenAPI spec, JSON schema, protocol definition files (see openapi.md)
│   ├── openapi.yaml  # Single source of truth for the REST contract
│   └── oapi-codegen.yaml
├── docs/
│   ├── adr/          # Architecture Decision Records
│   └── domain/       # Domain model, bounded context, and event diagrams (Mermaid)
├── migrations/       # SQL migration files
├── docker-compose.yml
└── DIAGRAM.md        # Top-level infrastructure diagram (Mermaid)
```

## Internal Layer Rules
- `domain` — pure Go structs, interfaces, and business logic; zero infrastructure imports
- `repository` — implements domain interfaces; owns all SQL, cache, and search I/O
- `app` — orchestrates domain + repository; owns transaction boundaries and outbox dispatch

## Testing
- Every package under `internal/` must have a `_test.go` file — see `.kiro/steering/testing.md`
- Unit tests use no external dependencies; integration tests are guarded by `//go:build integration`
- Fakes and test data builders live in `internal/domain/testutil/`

## Diagrams
- All diagrams are Mermaid, embedded in Markdown — see `.kiro/steering/diagramming.md`
- `DIAGRAM.md` is the infrastructure overview; `docs/domain/` holds domain-scoped diagrams

## Naming Conventions
- Packages: lowercase, single word
- Interfaces defined in `domain`, implemented in `repository`
- Errors: sentinel errors in `domain`, wrapped with `fmt.Errorf("%w", ...)` at boundaries

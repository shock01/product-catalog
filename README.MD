# Product Catalog — Reference Design

This is a **non-functional reference implementation** of an e-commerce product catalog backend. It is not production-ready and is not intended to be run as-is. The purpose is to demonstrate how to:

- model a complex domain correctly
- define the infrastructure and data architecture
- set up project steering so that an AI-assisted or team-based implementation stays consistent
- structure a Go service so the real implementation has a clear path forward

---

## What this is

A response to the following challenge:

> Design and build the backend for an e-commerce product catalog. The service exposes a REST API that lets clients create, retrieve, update, and delete products. The catalog holds a few hundred thousand products, reads vastly outnumber writes, and clients need to list and filter products (e.g. by category and price range) with pagination.

Rather than writing a minimal CRUD service, this project treats the challenge as a real production design problem and works through the decisions you would actually need to make.

---

## What is here

### Domain model
A fully worked-out Product aggregate with Options → Variants, full localisation (no human-readable text on root entities), StockStatus as an enum, and Channel/Marketplace as external reference tables. See [`docs/domain/product-aggregate.md`](docs/domain/product-aggregate.md).

### Architecture Decision Records
Nine ADRs covering every significant technical choice:

| ADR | Decision |
|---|---|
| [ADR-001](docs/adr/ADR-001-golang.md) | Go as the implementation language |
| [ADR-002](docs/adr/ADR-002-opensearch.md) | OpenSearch for search — one index per channel×locale, sequential worker |
| [ADR-003](docs/adr/ADR-003-postgres.md) | PostgreSQL over a document database |
| [ADR-004](docs/adr/ADR-004-valkey-bloom-filter.md) | Valkey cache + Bloom filter for GET by ID |
| [ADR-005](docs/adr/ADR-005-outbox-eventbridge.md) | Transactional outbox + EventBridge fan-out |
| [ADR-006](docs/adr/ADR-006-idempotency-optimistic-locking.md) | Idempotency keys + optimistic locking (If-Match / ETag) |
| [ADR-007](docs/adr/ADR-007-deployment.md) | AWS ECS Fargate — ECS Tasks triggered by EventBridge |
| [ADR-008](docs/adr/ADR-008-opentelemetry.md) | OpenTelemetry for traces, metrics, and logs |
| [ADR-009](docs/adr/ADR-009-domain-model.md) | Domain model — Product / Catalog bounded context |
| [ADR-010](docs/adr/ADR-010-indexer-design.md) | Indexer design — sequential worker, per-channel-locale indexes |
| [ADR-011](docs/adr/ADR-011-pricing-pipeline.md) | Pricing pipeline — Kafka consumer, S3 landing, price-importer, append-only snapshots |
| [ADR-012](docs/adr/ADR-012-inventory-pipeline.md) | Inventory pipeline — direct Postgres write, outbox, partial OpenSearch update |

### OpenAPI contract
A complete OpenAPI 3.1 spec at [`api/openapi.yaml`](api/openapi.yaml) covering two sub-contexts:
- `catalog/product` — CRUD backed by PostgreSQL (with Valkey cache)
- `catalog/search` — read-only search backed by OpenSearch

Commands return `202 Accepted` + HATEOAS links only (CQRS). Queries return `200` + full resource. ETags are content-derived (SHA-256). 410 Gone is distinguished from 404 Not Found.

### Database schema
Full PostgreSQL DDL in [`migrations/001_schema.sql`](migrations/001_schema.sql) with seed data in [`migrations/002_seed.sql`](migrations/002_seed.sql). Pricing schema in [`migrations/003_pricing.sql`](migrations/003_pricing.sql). Includes `snapshots` and `price_snapshot_imports` as write-guards, `price_snapshots` as an append-only price ledger, and `v_current_prices` / `v_products_current` as resolution views.

### Infrastructure diagrams
[`DIAGRAM.md`](DIAGRAM.md) contains Mermaid diagrams for:
- System overview (Athena + Kafka → S3 → Importer/price-importer → Postgres → EventBridge → Indexer ECS Tasks → OpenSearch)
- Single-product GET flow (Bloom filter → Valkey → Postgres)
- Mutation flow (optimistic locking + outbox)
- Athena snapshot import pipeline
- Pricing pipeline (Kafka → price-consumer → S3 → price-importer → Postgres → Indexer)
- Inventory pipeline (Kafka → inventory-consumer → Postgres → outbox → partial OpenSearch update)
- Indexer blue/green re-index per channel×locale
- Observability pipeline

### Local development stack
`compose.yaml` brings up the full infrastructure locally:

```
docker compose up
```

| Service | Port | Purpose |
|---|---|---|
| `postgres` | 5432 | Primary data store |
| `valkey` | 6379 | Cache + idempotency keys |
| `opensearch` | 9200 | Search index |
| `localstack` | 4566 | SQS + EventBridge (local AWS) |
| `otel-collector` | 4317 | OTLP gRPC receiver |
| `jaeger` | 16686 | Trace UI |
| `prometheus` | 9090 | Metrics UI |

Migrations run automatically via `dbmate` before the app services start.

### Kiro steering files
`.kiro/steering/` contains ten steering documents that encode the design decisions as rules for AI-assisted development. They cover language and layout, data model, infrastructure, API design, operational patterns, diagramming standards, OpenAPI location, URL structure, testing standards, and CQRS principles. Any AI assistant working on this codebase will pick these up automatically.

### Skeleton Go code
`internal/domain/`, `internal/repository/postgres/`, and `internal/app/` contain the domain entities, repository interface, and application service layer — enough to show the layering and the shape of the real implementation without pretending to be complete.

---

## Key design choices worth noting

**Reads vastly outnumber writes → separate read and write paths.** PostgreSQL owns the write path and single-product reads (via Valkey cache). OpenSearch owns the list/search path. These are independent scaling axes.

**Per-channel × locale OpenSearch indexes.** Rather than one shared index with runtime filtering, each channel+locale combination gets its own pre-projected index (`products-web-en-gb`, `products-pos-de-de`, etc.). Documents are fully denormalised at index time — no runtime joins, no locale resolution in the query. Index count grows as `channels × locales` but stays manageable (e.g. 3 × 5 = 15 indexes).

**Sequential indexer, not a parallel queue.** A single Go worker reading Postgres in batches of 500 and bulk-writing to OpenSearch handles hundreds of thousands of products in 1–2 minutes. Parallelising writers to the same OpenSearch index causes segment merge pressure and write throttling — it is slower, not faster. The right unit of parallelism is across indexes (one ECS Task per channel×locale), not within one.

**Athena snapshot import with a write-guard.** Products are imported from Athena-generated Parquet files on S3. Each import run is recorded as a `snapshot` row. Products and variants carry a `snapshot_id` FK; the upsert only overwrites a row when the incoming snapshot's `dumped_at` is strictly newer. Re-running the same dump is a no-op. API mutations never set `snapshot_id` and are never overwritten by an import.

**Append-only pricing with a view-based resolution.** Prices are per product × channel × marketplace and are never updated in place — each import appends new rows to `price_snapshots`. The current price is resolved by the `v_current_prices` view using specificity ranking (channel+marketplace > channel > marketplace > global) and `effective_from` ordering. The view falls back to legacy `channel_products.price_override` and `variants.price` if no snapshot exists. The indexer and API query the view — resolution logic lives in SQL once, not in Go.

**Kafka consumer lands to S3; price-importer is stateless.** The `price-consumer` ECS Service consumes the external `pricing.price-changed` Kafka topic and batches records to S3 as JSON. The `price-importer` ECS Task is triggered by S3 ObjectCreated events and handles both Kafka stream landings and Athena full-price dumps identically. Kafka offsets are committed only after the S3 write succeeds.

**Inventory pipeline — direct write, no S3 landing.** The `inventory-consumer` ECS Service consumes `inventory.stock-changed` Kafka events and writes directly to Postgres in a single transaction: upsert `stock_levels` + outbox row. The `IS DISTINCT FROM` guard prevents spurious outbox events when the same level arrives twice. Stock changes trigger a partial OpenSearch `_update` on the affected product document(s) — not a full re-index — keeping propagation fast.

**ECS Tasks triggered by EventBridge, not always-on consumers.** The importer and indexer only need to run when there is new data. EventBridge rules fire ECS Tasks on `s3:ObjectCreated` (importer) and `snapshot.imported` (indexer, one rule per channel×locale). No SQS queue, no visibility timeout tuning, no dead-letter queue management.

**CQRS on the API.** Mutating operations (POST, PUT, PATCH, DELETE) return `202 Accepted` with HATEOAS links only — no resource body. Queries (GET) return the full resource. This keeps the command and query paths cleanly separated and makes the API safe to retry.

---

## What is not here

- A working HTTP server (the `cmd/api/main.go` wiring)
- A working indexer binary (`cmd/indexer/main.go`)
- A working importer binary (`cmd/importer/main.go`)
- A working price-consumer binary (`cmd/price-consumer/main.go`) — Kafka client, S3 batch writer
- A working price-importer binary (`cmd/price-importer/main.go`) — S3 JSON reader, price snapshot upsert
- A working inventory-consumer binary (`cmd/inventory-consumer/main.go`) — Kafka client, direct Postgres upsert + outbox
- OpenSearch document mapping and query construction
- Bloom filter implementation
- Outbox worker goroutine
- OAuth2 middleware
- Tests

These are the natural next steps for a real implementation, and the steering files, domain model, schema, and ADRs give a clear enough foundation to build them without ambiguity.

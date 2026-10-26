# Operational Patterns

## S3 Import Pipeline (Athena Snapshots)
- Athena runs a scheduled query and `UNLOAD`s results to `s3://bucket/catalog/snapshots/{dumped_at}/products.parquet`
- S3 event notification triggers an SQS message to the `product-catalog-importer` queue
- The Importer Job (`cmd/importer`) consumes the message, reads the Parquet file from S3, and upserts into Postgres
- Before writing, it inserts a `snapshots` row (`status='processing'`), then applies the **snapshot guard** on every product/variant upsert (see `data-model.md` § Snapshot Guard)
- On completion: updates `snapshots.status='completed'`, writes an `outbox` row `snapshot.imported`
- The outbox worker fans out to EventBridge → indexer SQS queue → full or incremental re-index
- A re-run of the same dump is safe: equal `dumped_at` means all upserts are no-ops
- API mutations (`PUT`/`PATCH`) do not set `snapshot_id`; those rows are never overwritten by an import

## Outbox Pattern
- Every mutation (create, update, delete) writes an `outbox` row in the same Postgres transaction
- A background goroutine polls `outbox WHERE dispatched_at IS NULL ORDER BY created_at LIMIT 100`
- Rows are published to EventBridge, then marked `dispatched_at = now()`
- Guarantees at-least-once delivery; consumers must be idempotent

## Stock Level Updates
- A separate consumer listens to `stock.level_changed` events from EventBridge/SQS
- The event carries a `StockStatus` value: `IN_STOCK`, `LOW`, or `OOS` — never a raw quantity
- On receipt: update `stock_levels.level` in Postgres (optimistic lock on version)
- Write an outbox event `product.stock_updated` (or `product.stock_depleted` / `product.stock_restored` on OOS transitions)
- Trigger an incremental OpenSearch update for the affected product (via the indexer SQS queue)

## Indexer Job (Sequential, Per-Channel-Locale, ECS Task)
See ADR-010 for the full rationale. Key points:

- **One index per channel × locale**: `products-{channel}-{locale}-{timestamp}`, with a stable alias `products-{channel}-{locale}` that the API queries
- **No internal queue**: a single sequential Go worker reads Postgres in batches of 500 and bulk-writes to OpenSearch. At ~5k–10k docs/sec this handles hundreds of thousands of products in 1–2 minutes — fast enough for a batch import pattern
- **No parallelism within one index**: concurrent bulk writers to the same index cause segment merge pressure and write throttling. The right unit of parallelism is across indexes (channel×locale), not within one
- **ECS Task per channel×locale**: triggered by EventBridge rule on `snapshot.imported`. Tasks run concurrently across combinations; each writes to a different index so there is no contention
- **Blue/green swap**: build new index, bulk-load, swap alias atomically, delete old index. A failed job leaves the alias on the previous index — safe fallback
- **Eligibility rules** (in `domain/indexrules.go`):
  - `status = 'published'`
  - At least one variant where `stock_levels.level != 'OOS'`
  - `channel_products.enabled = true` for the target channel

## Bloom Filter Lifecycle
- Loaded from Postgres (`SELECT id FROM products WHERE deleted_at IS NULL`) at service startup
- Refreshed every 5 minutes via a background goroutine
- On product creation: add ID to in-memory filter immediately
- On product deletion: filter is eventually consistent (false positives allowed; Postgres is authoritative)

## Observability (see ADR-008)
- Single instrumentation layer: OpenTelemetry Go SDK for traces, metrics, and logs
- **Traces**: auto-instrument HTTP, pgx, Valkey, OpenSearch, SQS/EventBridge spans; propagate W3C `traceparent` on all inbound/outbound calls; embed `trace_id`+`span_id` in outbox event payloads for cross-service linking
- **Metrics**: dual-export via Prometheus scrape (`/metrics`) and OTLP push to OTel Collector
  - `http_request_duration_seconds`, `bloom_filter_hits_total`, `bloom_filter_misses_total`
  - `cache_hits_total`, `cache_misses_total`, `outbox_pending_total`, `outbox_dispatch_duration_seconds`
  - `index_job_duration_seconds` (labelled `mode=full|incremental`), `opensearch_index_doc_count`
- **Logs**: `log/slog` with OTel handler that injects `trace_id`+`span_id` into every record; structured JSON only; level via `LOG_LEVEL` env var
- **Collector**:
  - Local: `otel-collector` container in docker-compose → Jaeger (traces) + Prometheus (metrics)
  - Production: OTel Collector as ECS sidecar or K8s DaemonSet → AWS X-Ray + Amazon Managed Prometheus / CloudWatch

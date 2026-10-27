# ADR-011: Pricing Pipeline — Kafka Consumer, S3 Landing, Price Importer, Append-Only Snapshots

## Status
Accepted

## Context
Pricing is a separate bounded context from the product catalog. Prices change frequently and are owned by an external pricing service that publishes to a Kafka topic. We also need a full-price import path from Athena for bulk loads and backfills. Prices must be reflected in the OpenSearch index so search results show current prices.

## Decisions

### 1. Price is per product × channel × marketplace

A product can have different prices on the web channel vs POS, and different prices on Amazon UK vs Amazon DE. The most specific price wins at resolution time. `channel_id NULL` means the price applies to all channels; `marketplace_id NULL` means all marketplaces. Resolution priority:

```
channel + marketplace  (specificity 3)
channel only           (specificity 2)
marketplace only       (specificity 1)
global (both NULL)     (specificity 0)
```

### 2. Append-only price_snapshots table

Prices are never updated in place. Each import appends new rows. The current price is always the row with the latest `effective_from <= now()` for a given `product_id × channel_id × marketplace_id`. This gives a full audit trail and makes backfills safe — inserting a historical price does not overwrite the current one.

### 3. Write-guard via price_snapshot_imports

Mirrors the `snapshots` write-guard pattern. Each S3 file processed is recorded as a `price_snapshot_imports` row. The importer only inserts a new `price_snapshots` row if the incoming `effective_from` is strictly newer than the latest existing row for that combination. Re-running the same file is a no-op.

### 4. v_current_prices view

The resolution logic (specificity ranking + effective_from ordering + legacy fallback) lives in a single Postgres view. The API, indexer, and any future consumer query this view — they never implement resolution logic themselves. The view falls back to `channel_products.price_override` then `variants.price` if no price snapshot exists, preserving backward compatibility.

### 5. v_products_current view

A view over `products` joined to their latest `snapshots` row. The API's `GetByID` and the indexer query this view instead of `products` directly. Snapshot resolution (which snapshot is "current") is expressed once in SQL, not scattered across Go code.

### 6. Two ingestion paths, one importer binary

Both paths produce JSON files on S3 and trigger the same `price-importer` ECS Task:

```
Path A — Kafka stream:
  External Kafka topic: pricing.price-changed
    → price-consumer (ECS Service, long-running)
        batches records (5 min window or 1,000 records)
        writes JSON to s3://bucket/pricing/stream/{date}/{hour}/{uuid}.json
        → S3 ObjectCreated → EventBridge → ECS RunTask: price-importer

Path B — Athena full import:
  Athena scheduled query
    → UNLOAD to s3://bucket/pricing/snapshots/{dumped_at}/prices.json
    → S3 ObjectCreated → EventBridge → ECS RunTask: price-importer
```

The `price-importer` binary receives `S3_BUCKET` and `S3_KEY` env vars and does not care which path produced the file.

### 7. Kafka consumer is a long-running ECS Service

Unlike the importer and indexer (ECS Tasks that exit when done), the Kafka consumer must maintain consumer group membership continuously. It runs as an ECS Service with a single task. It does not write to Postgres directly — it only lands JSON to S3, keeping its responsibilities minimal and its failure domain isolated.

### 8. JSON format for price events

Parquet is appropriate for large Athena exports of structured product data. For the pricing stream, JSON is simpler: the Kafka messages are already JSON, the consumer writes them as JSON lines, and the importer reads them without a Parquet dependency. The volume per batch is small (≤1,000 records × ~200 bytes = ~200KB).

### 9. Re-indexing triggered by price.imported

The price-importer emits a `price.imported` outbox event after committing. EventBridge routes this to the same indexer ECS Tasks (one per channel×locale) that handle `snapshot.imported`. The indexer reads prices from `v_current_prices` — no indexer code changes are needed when prices change.

## JSON event schema (Kafka → S3)

```json
{
  "product_id": "uuid",
  "channel_id": "uuid | null",
  "marketplace_id": "uuid | null",
  "amount": "19.99",
  "currency": "GBP",
  "effective_from": "2024-01-15T12:00:00Z"
}
```

## Consequences

- `price_snapshots` grows indefinitely — a periodic cleanup job should archive rows older than the retention window (e.g. 90 days) to cold storage
- The `v_current_prices` view uses window functions; for very high-cardinality queries the indexer should materialise prices per batch rather than joining the view per product
- Adding a new channel or marketplace requires no schema change — the NULL-based resolution handles it automatically
- The Kafka consumer is the only always-on component added; all other new components are ECS Tasks

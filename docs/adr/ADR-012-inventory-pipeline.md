# ADR-012: Inventory Pipeline — Direct Postgres Write, Outbox, Partial OpenSearch Update

## Status
Accepted

## Context
Stock levels change frequently and are owned by an external inventory service that publishes to a Kafka topic. The catalog needs current stock status per variant × warehouse to enforce index eligibility and surface availability in search results.

## Decision
The `inventory-consumer` ECS Service consumes Kafka directly and writes to Postgres in a single transaction per record. No S3 landing, no separate importer Task. Stock changes propagate to OpenSearch via the outbox as a **partial document update**, not a full re-index.

## Why no S3 landing

The pricing pipeline uses S3 because the Athena full-dump path forces it — both ingestion paths (stream + Athena) share one importer binary, so S3 is the natural handoff. Inventory has no Athena path. Adding S3 would mean:

- Two ECS components (consumer + importer Task) where one does the job
- A 5-minute batch window adding unnecessary latency to stock updates
- An EventBridge S3 rule and a separate Task definition for no correctness benefit

The `IS DISTINCT FROM` guard on the upsert already makes the write idempotent, so at-least-once Kafka delivery is safe without S3 replay.

## Why no full re-index

A stock level change on one variant should not trigger a blue/green rebuild of the entire channel×locale index. That is expensive (minutes) and disproportionate for a single field update. Instead, the indexer handles `stock.updated` as a **partial update**: a targeted `POST /{index}/_update/{product_id}` that patches only the stock fields on the affected document(s).

## Why outbox, not a direct OpenSearch write

Writing to both Postgres and OpenSearch in the consumer would be a dual-write — if OpenSearch is temporarily unavailable, the Postgres commit succeeds but the index is stale with no recovery path. The outbox gives atomicity: the stock upsert and the outbox row commit together, and the outbox worker retries delivery until OpenSearch acknowledges the update.

## Write path (per Kafka record)

```sql
BEGIN;

INSERT INTO stock_levels (variant_id, warehouse_code, level, updated_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (variant_id, warehouse_code)
DO UPDATE SET level      = EXCLUDED.level,
              version    = stock_levels.version + 1,
              updated_at = now()
WHERE stock_levels.level IS DISTINCT FROM EXCLUDED.level;

-- only if rows_affected > 0 (level actually changed)
INSERT INTO outbox (aggregate, aggregate_id, event_type, payload)
VALUES ('variant', $variant_id, 'stock.updated',
        '{"variant_id": $1, "warehouse_code": $2, "level": $3}');

COMMIT;
```

Kafka offset is committed only after the Postgres transaction succeeds.

## Indexer partial update (stock.updated handler)

The indexer already handles `snapshot.imported` and `price.imported` as full re-indexes. It adds a third handler for `stock.updated`:

1. Look up the `product_id` for the affected `variant_id`
2. Recompute the product's aggregate stock status across all variants and warehouses
3. `POST /{index}/_update/{product_id}` with the updated stock fields
4. If the product transitions to fully OOS, remove it from the index (`DELETE /{index}/_doc/{product_id}`)

This runs against all channel×locale indexes for the affected product — typically 15 or fewer requests.

## Consequences

- `inventory-consumer` is the only always-on component added for inventory
- No new tables — `stock_levels` already has the right shape; `updated_at` added in `migrations/004_inventory.sql`
- The `IS DISTINCT FROM` guard prevents spurious outbox events when the same level arrives twice
- Partial OpenSearch updates are faster than full re-indexes but require the indexer to handle a new event type
- A product that transitions OOS must be removed from the index; one that transitions back must be re-added — the partial update handler must cover both cases

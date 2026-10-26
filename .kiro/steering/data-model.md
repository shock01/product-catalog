# Data Model

## Relational Model (Postgres)
The catalog is split into normalised tables. Never use a single "product" blob.

### Core Tables
| Table | Purpose |
|---|---|
| `channels` | **Reference table** — local projection of Channel Service data; not owned by Catalog |
| `marketplaces` | **Reference table** — local projection of Marketplace Service data; not owned by Catalog |
| `snapshots` | One row per Athena dump file processed; carries `s3_uri`, `dumped_at`, `status`, `row_count` |
| `products` | Master product record (status, timestamps, version, `snapshot_id`) — **no SKU, no human-readable text** |
| `options` | Locale-neutral variation axes (e.g. code=`size`) |
| `option_translations` | Localised option names per locale (e.g. "Size", "Größe") |
| `option_values` | Locale-neutral selectable values (e.g. code=`xl`, `red`) |
| `option_value_translations` | Localised option value labels per locale (e.g. "XL", "Rot") |
| `variants` | Concrete purchasable unit; carries the **SKU**, price, currency, and `snapshot_id` |
| `variant_option_values` | Join: which option values define a given variant |
| `assets` | Images and media linked to product or variant |
| `locales` | Supported locale codes |
| `product_translations` | `title`, `slug`, `description`, `short_description`, `meta_title`, `meta_description` per locale |
| `variant_translations` | Localised `title` and `description` per locale |
| `channel_products` | Visibility and price overrides per channel (references `channels.id`) |
| `marketplace_products` | Marketplace-specific metadata (references `marketplaces.id`) |
| `stock_levels` | Variant stock status per warehouse/location (`IN_STOCK`, `LOW`, `OOS`) |

### Snapshot Guard (Upsert Rule)
`products.snapshot_id` and `variants.snapshot_id` reference the `snapshots` row that last wrote each record. The importer only overwrites a row when the incoming snapshot's `dumped_at` is **strictly greater** than the stored snapshot's `dumped_at`:

```sql
INSERT INTO products (..., snapshot_id)
VALUES (..., $new_snapshot_id)
ON CONFLICT (id) DO UPDATE
  SET ..., snapshot_id = EXCLUDED.snapshot_id
  WHERE (SELECT dumped_at FROM snapshots WHERE id = EXCLUDED.snapshot_id)
      > (SELECT dumped_at FROM snapshots WHERE id = products.snapshot_id);
```

This means:
- Re-running the same dump is a **no-op** (equal `dumped_at`).
- An older dump arriving late cannot overwrite a newer import.
- API mutations (PUT/PATCH) do **not** set `snapshot_id`; those rows are considered "manually managed" and are never overwritten by an import.

### Reference Table Sync
`channels` and `marketplaces` are **not owned by the Catalog**. They are kept in sync from external services via one of two strategies:
- **Event stream** (preferred): consume `channel.created/updated` and `marketplace.created/updated` events from Kafka or Kinesis and upsert locally
- **API import** (fallback): a scheduled job calls the owning service's REST API and upserts the result

See ADR-009 § Foreign References for the full decision.

### Views
Create a Postgres view per channel × locale combination, e.g.:
- `v_product_web_en_gb` — joins product + translations + channel_products + assets for web / en-GB
- Views are the read projection used by the API list/filter endpoints

### Versioning & Optimistic Locking
- Every mutable table carries a `version INTEGER NOT NULL DEFAULT 1`
- All UPDATE statements include `WHERE id = $1 AND version = $2`
- The HTTP layer enforces `If-Match: "<version>"` on PUT/PATCH; returns 412 on mismatch

### Outbox Table
```sql
CREATE TABLE outbox (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  aggregate   TEXT NOT NULL,   -- e.g. "product"
  aggregate_id UUID NOT NULL,
  event_type  TEXT NOT NULL,   -- e.g. "product.updated"
  payload     JSONB NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  dispatched_at TIMESTAMPTZ
);
```
Outbox rows are written in the same transaction as the mutation and polled by a background worker.

## Why Relational (see ADR-003)
A document store cannot efficiently serve all required access patterns:
- Filter by price range + category + locale + channel simultaneously
- Aggregate stock across variants
- Transactional outbox in the same write
- Normalised locale/channel overrides without data duplication

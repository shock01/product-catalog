# ADR-002: OpenSearch for Product Search and Listing

## Status
Accepted

## Context
The catalog holds hundreds of thousands of products. Clients need to filter by category, price range, locale, and channel, with full-text search and cursor-based pagination. These queries are read-heavy and need sub-100ms p99 latency.

## Decision
Use OpenSearch as the search and list backend. All `GET /catalog/search/v1` (list/filter) requests are served from OpenSearch. Single-product `GET /catalog/product/v1/{id}` is served from Postgres (with Valkey cache).

## Rationale
- Inverted index makes full-text and faceted filtering orders of magnitude faster than Postgres `LIKE` or even GIN indexes at this data volume
- Horizontal scaling of read replicas without Postgres replication lag concerns
- Native support for cursor pagination via `search_after`
- Aggregations for facet counts (category counts, price histograms) at no extra query cost
- Decouples read scaling from write scaling

## Index Strategy

### One index per channel × locale
Rather than a single `products-active` alias, we maintain one index per channel × locale combination:

```
products-{channel}-{locale}-{timestamp}   e.g. products-web-en-gb-20240115120000
```

Each index contains fully denormalised, pre-projected documents for that specific channel and locale — title, slug, price override, stock status, option value labels — all resolved at index time. The API search context queries the index for the requested channel+locale directly. No runtime joins, no locale/channel filtering in the query.

An alias per channel×locale points to the current live index:
```
products-web-en-gb  →  products-web-en-gb-20240115120000
products-web-de-de  →  products-web-de-de-20240115120000
products-pos-en-gb  →  products-pos-en-gb-20240115120000
```

### Sequential single-worker re-index (no queue)
A few hundred thousand products does not require a distributed queue or parallel writers. A single Go worker reading from Postgres in batches of 500 and bulk-writing to OpenSearch will complete a full re-index in 2–5 minutes. This is fast enough.

Parallelising OpenSearch writers would be actively harmful: concurrent bulk requests to the same index cause segment pressure, merge storms, and write throttling — the result is slower, not faster. The right unit of parallelism is **across indexes** (channel×locale), not within one.

### Blue/green swap per index
Each indexer job builds a new `products-{channel}-{locale}-{timestamp}` index, bulk-loads all eligible documents, then atomically swaps the alias. The old index is deleted after the swap. A failed job leaves the alias pointing at the previous index — safe fallback with no downtime.

## Business Eligibility Rules
Not all products in Postgres are indexed. A product is indexed only if:
1. `status = 'published'`
2. At least one variant has `stock_levels.level != 'OOS'` across any warehouse
3. `channel_products.enabled = true` for the target channel

These rules live in `internal/domain/indexrules.go` and are applied by every indexer job.

## Consequences
- Index count grows as `channels × locales` — manageable (e.g. 3 channels × 5 locales = 15 indexes)
- Each index is smaller and cheaper to query than a single combined index with runtime filtering
- Eventual consistency between Postgres and OpenSearch (minutes for full re-index; acceptable given the import-driven write pattern)
- A failed re-index for one channel×locale does not affect others

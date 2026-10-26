# ADR-004: Valkey Cache with Bloom Filter for Single-Product GET

## Status
Accepted

## Context
Single-product `GET /v1/products/{id}` is the highest-volume endpoint. Reads vastly outnumber writes. We need to protect Postgres from both legitimate read load and out-of-service / enumeration attacks where clients request IDs that do not exist, causing cache misses that fall through to the database.

## Decision
Place a Valkey (Redis-compatible) cache in front of Postgres for single-product lookups, and load a Bloom filter at startup to short-circuit requests for non-existent IDs before they reach the cache or database.

## Bloom Filter Design
- At startup, load all known product IDs from Postgres into an in-memory Bloom filter (`github.com/bits-and-blooms/bloom/v3`)
- Estimated false-positive rate: 0.1% at 500k products with ~1.2MB memory
- A GET for an ID that the Bloom filter says is absent → return `404` immediately, no cache or DB hit
- A GET for an ID the filter says may exist → check Valkey, then Postgres on miss
- Filter is refreshed every 5 minutes from Postgres; new products are added immediately on creation
- False positives (filter says exists, but product was deleted) are handled by the cache/DB returning 404 — acceptable

## Cache Design
- Cache key: `product:{id}` (serialised JSON of the full product response)
- TTL: 5 minutes (configurable via `CACHE_TTL_SECONDS`)
- Write-through invalidation: on any mutation, delete `product:{id}` from Valkey
- Valkey chosen over Redis for licence compatibility (BSD); API is identical

## Consequences
- Bloom filter adds ~1–2ms to startup time for 500k IDs — acceptable
- False-positive rate means ~0.1% of non-existent ID requests still hit the cache/DB — acceptable
- Cache invalidation on write adds a small latency overhead to mutations — acceptable
- Valkey must be treated as ephemeral; Postgres is always authoritative

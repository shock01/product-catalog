# ADR-006: Idempotent Mutations with Optimistic Locking

## Status
Accepted

## Context
The catalog API is consumed by multiple clients and automation pipelines. Network retries and concurrent updates can cause duplicate writes or lost updates. We need a mechanism to make mutations safe to retry and to prevent concurrent overwrites.

## Decision
1. All mutating endpoints require an `If-Match: "<version>"` header containing the current ETag of the resource
2. POST (create) accepts an optional `Idempotency-Key` header
3. Optimistic locking is enforced at the database level via a `version` column

## Optimistic Locking
- Every mutable table has `version INTEGER NOT NULL DEFAULT 1`
- GET responses include `ETag: "<version>"` header
- PUT/PATCH/DELETE include `If-Match: "<version>"` in the request
- The UPDATE statement is: `UPDATE products SET ..., version = version + 1 WHERE id = $1 AND version = $2`
- If `rows_affected == 0`: return `412 Precondition Failed` with body `{"error": {"code": "VERSION_CONFLICT"}}`
- Clients must re-fetch (GET) and retry with the new ETag

## Idempotency Keys (POST)
- Client sends `Idempotency-Key: <uuid>` on POST
- Server checks Valkey for key `idem:{key}` (TTL 24h)
- If found: return the stored response (same status code + body) without re-processing
- If not found: process, store response in Valkey, return response
- Key is scoped to the endpoint path to prevent cross-endpoint collisions

## Rationale
- Optimistic locking avoids pessimistic locks (no `SELECT FOR UPDATE`) which would serialise all writes
- Version-based ETags are standard HTTP (RFC 7232) — clients can use standard HTTP libraries
- Idempotency keys make POST safe to retry on network failure without creating duplicate products
- Both mechanisms together make the entire mutation surface safe for at-least-once delivery from upstream event consumers

## Consequences
- Clients must implement ETag round-trip for updates — slightly more complex than a simple PUT
- High-contention products (e.g. flash sale stock updates) may see elevated 412 rates; clients need exponential backoff
- Idempotency key store in Valkey is ephemeral; a Valkey restart within the 24h window could allow duplicate POSTs — acceptable risk

# ADR-006: Idempotent Mutations with Optimistic Locking

## Status
Accepted

## Context
The catalog API is consumed by multiple clients and automation pipelines. Network retries and concurrent updates can cause duplicate writes or lost updates. We need a mechanism to make mutations safe to retry and to prevent concurrent overwrites.

## Decision
1. All mutating endpoints require an `If-Match: "<etag>"` header containing the current ETag of the resource
2. Optimistic locking is enforced at the database level via a `version` column

## Optimistic Locking
- Every mutable table has `version INTEGER NOT NULL DEFAULT 1`
- GET responses include an `ETag: "<etag>"` header (content-derived: SHA-256 of canonical JSON, first 16 hex chars)
- PUT/PATCH/DELETE send `If-Match: "<etag>"` in the request
- The UPDATE statement is: `UPDATE products SET ..., version = version + 1 WHERE id = $1 AND version = $2`
- If `rows_affected == 0`: return `412 Precondition Failed` with body `{"error": {"code": "VERSION_CONFLICT"}}`
- Clients must re-fetch (GET) and retry with the new ETag

## Rationale
- Optimistic locking avoids pessimistic locks (no `SELECT FOR UPDATE`) which would serialise all writes
- Content-derived ETags are standard HTTP (RFC 7232) — clients can use standard HTTP libraries
- The ETag changes whenever any field changes, so stale reads are always detected

## Consequences
- Clients must round-trip the `ETag` / `If-Match` headers for updates — slightly more complex than a simple PUT
- High-contention products (e.g. flash sale stock updates) may see elevated 412 rates; clients need exponential backoff

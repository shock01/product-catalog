# ADR-003: PostgreSQL as the Primary Data Store

## Status
Accepted

## Context
We need a primary store for product data that supports the full range of access patterns required by the catalog, including mutations, transactional outbox, and complex relational queries.

## Decision
Use PostgreSQL as the primary (write) data store.

## Why Not a Document Database?

A document store (MongoDB, DynamoDB, Firestore, etc.) was considered and rejected for the following reasons:

| Access Pattern | Document DB | Postgres |
|---|---|---|
| Filter by price range + category + locale + channel simultaneously | Requires compound indexes per combination; explodes at scale | Single query with joins and indexes |
| Aggregate stock across variants | Requires application-side aggregation or denormalisation | `SUM(stock_levels.quantity) GROUP BY product_id` |
| Transactional outbox (mutation + event in one transaction) | Not supported in most document DBs without multi-document transactions (expensive) | Native ACID transaction |
| Normalised locale/channel overrides | Forces duplication of the full document per locale/channel | Separate `product_translations` and `channel_products` tables |
| Optimistic locking with version field | Possible but requires careful application logic | `WHERE version = $n` in UPDATE, enforced at DB level |
| Schema evolution with migrations | Schema-less flexibility is a liability at scale | `golang-migrate` gives auditable, reversible migrations |

## Rationale
- ACID transactions are required for the outbox pattern — the mutation and the outbox row must commit atomically
- The data model is inherently relational: products have variants, variants have stock levels, products have translations per locale, pricing varies per channel
- Postgres `JSONB` columns are available for truly schemaless attributes (e.g. variant-specific custom attributes) without abandoning relational structure
- Mature ecosystem, excellent `pgx` driver for Go, strong operational tooling

## Consequences
- Vertical scaling of the write path is bounded; mitigated by read replicas for reporting queries and OpenSearch for search
- Schema changes require migrations — acceptable cost for the correctness guarantees gained
- Connection pooling (PgBouncer or `pgx` pool) required at scale

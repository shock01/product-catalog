# ADR-005: Outbox Pattern with EventBridge Fan-Out

## Status
Accepted

## Context
Product mutations must reliably trigger downstream side-effects: updating the OpenSearch index, notifying other services (stock, pricing, recommendations). We cannot use dual-writes (write to DB then publish to a queue) because a crash between the two leaves the system inconsistent.

## Decision
Use the Transactional Outbox pattern. Every mutation writes an `outbox` row in the same Postgres transaction. A background worker polls the outbox and publishes events to AWS EventBridge. EventBridge fans out to SQS queues consumed by downstream services.

## Flow
```
Mutation handler
  └─ BEGIN TRANSACTION
       ├─ UPDATE products SET ...
       └─ INSERT INTO outbox (aggregate, aggregate_id, event_type, payload)
     COMMIT

Outbox worker (background goroutine, polls every 500ms)
  └─ SELECT * FROM outbox WHERE dispatched_at IS NULL LIMIT 100
       ├─ PutEvents → EventBridge bus "product-catalog"
       └─ UPDATE outbox SET dispatched_at = now() WHERE id = $1

EventBridge rules
  ├─ → SQS: product-catalog-indexer   (consumed by indexer job)
  ├─ → SQS: product-catalog-stock     (consumed by stock service)
  └─ → SQS: product-catalog-*         (any future consumer)
```

## Event Schema
```json
{
  "source": "product-catalog",
  "detail-type": "product.updated",
  "detail": {
    "product_id": "...",
    "version": 4,
    "changed_fields": ["price", "stock_level"]
  }
}
```

## Rationale
- Atomicity: outbox row and mutation commit together — no lost events
- At-least-once delivery: worker retries until `dispatched_at` is set; consumers must be idempotent
- EventBridge decouples producers from consumers — new consumers subscribe without changing the catalog service
- SQS provides buffering and retry for each consumer independently

## Consequences
- Outbox table grows; a cleanup job must purge rows older than N days
- Polling adds ~500ms latency to event delivery (acceptable; incremental indexer handles near-real-time needs)
- EventBridge has a 256KB event size limit; large payloads must use a reference pattern (store in S3, put S3 key in event)

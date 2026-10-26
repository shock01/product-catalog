# Infrastructure

## Local Development
- Entry point is `compose.yaml` at the repo root — run `docker compose up`
- Uses Compose `include` to split concerns:
  - `compose/infra.yaml` — all infrastructure services (Postgres, Valkey, OpenSearch, LocalStack, OTel Collector, Jaeger, Prometheus, **migrate**)
  - `compose/apps.yaml` — application services (api, indexer)
- The `migrate` service (dbmate) runs schema migrations and seed data, then exits; app services depend on `service_completed_successfully`
- Supporting configs: `compose/otel-collector.yaml`, `compose/prometheus.yaml`

## Postgres (Primary Store)
- Single-writer relational database for all product mutations
- Migrations managed with **dbmate** (`ghcr.io/amacneil/dbmate:2`) under `migrations/`
- Migration files: `migrations/001_schema.sql` (DDL), `migrations/002_seed.sql` (reference data + sample products)
- Connection pool via `pgx/v5`

## Valkey Cache (Read-Through + Bloom Filter)
- Sits in front of Postgres for single-product GET by ID
- Bloom filter loaded at startup (and refreshed periodically) with all known product IDs
- A GET for an unknown ID is rejected at the bloom filter layer before hitting Postgres or cache
  — protects against cache-miss DoS / out-of-service attacks
- Cache TTL: configurable, default 5 minutes
- Cache key: `product:{id}`
- On write: invalidate cache key and update bloom filter

## OpenSearch (Search & List)
- Used for all list/filter/search operations (category, price range, full-text, pagination)
- Index name pattern: `products-{timestamp}` with an alias `products-active`
- A dedicated **indexer job** (`cmd/indexer`) rebuilds the index from Postgres and performs a blue/green alias swap — zero downtime re-index
- Business rules in `domain` determine which products are eligible for indexing (published, in-stock threshold, channel visibility)
- The indexer also listens to the SQS change-event queue to apply incremental updates between full re-indexes

## SQS + EventBridge (Events)
- Every product mutation dispatches a domain event to an SQS queue via the outbox worker
- The outbox worker polls the `outbox` table, publishes to EventBridge, marks rows dispatched
- EventBridge routes events to downstream consumers (indexer, stock service, etc.)
- Queue names follow the pattern `product-catalog-{env}-events`

## Deployment
- Containerised as Docker images
- Deployable to Kubernetes (Deployment + Service + HPA) or AWS ECS (Fargate task definitions)
- Config via environment variables; secrets via AWS Secrets Manager or Kubernetes Secrets
- Health endpoints: `GET /healthz` (liveness) and `GET /readyz` (readiness)

## API Contract
- Defined in `api/openapi.yaml` (OpenAPI 3.1)
- Code-generate server stubs and request/response types from the spec using `oapi-codegen`
- The spec is the source of truth — never hand-write handler signatures

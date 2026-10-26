# ADR-007: Deployment on Kubernetes / AWS ECS

## Status
Amended (supersedes indexer section; see ADR-010 for indexer design)

## Context
We need a deployment target that supports horizontal scaling, rolling deploys, health-based routing, and secrets management. The service consists of three binaries: `api`, `indexer`, and `importer`.

## Decision
Package all binaries as Docker images. Deploy on AWS ECS Fargate.

## Container Design
- Single `Dockerfile` per binary under `cmd/api/`, `cmd/indexer/`, `cmd/importer/`
- Multi-stage build: `golang:1.23-alpine` builder → `gcr.io/distroless/static` runtime
- No shell in production image; secrets injected via environment variables

## AWS ECS Fargate
- `api` — long-running ECS Service behind an ALB target group; scales on CPU + request rate
- `importer` — ECS Task (run-to-completion); triggered by EventBridge rule on S3 `s3:ObjectCreated` event for the snapshot prefix
- `indexer` — ECS Task (run-to-completion) per channel×locale; triggered by EventBridge rule on `snapshot.imported` domain event published via the outbox

### Indexer trigger chain
```
S3 snapshot uploaded
  → EventBridge S3 rule
    → ECS RunTask: importer
      → importer completes, writes outbox event snapshot.imported
        → outbox worker publishes to EventBridge bus
          → EventBridge rule per channel×locale
            → ECS RunTask: indexer (CHANNEL=web, LOCALE=en-GB)
            → ECS RunTask: indexer (CHANNEL=web, LOCALE=de-DE)
            → ECS RunTask: indexer (CHANNEL=pos, LOCALE=en-GB)
            → ... one task per combination
```

Each indexer task receives its target channel and locale via environment variables (`INDEX_CHANNEL`, `INDEX_LOCALE`). Tasks run concurrently — they write to different OpenSearch indexes so there is no contention.

## Kubernetes (alternative)
- `api` as a `Deployment` with HPA
- `importer` and `indexer` as `Job` objects triggered by an event-driven operator (e.g. KEDA with EventBridge source)
- Liveness: `GET /healthz`, Readiness: `GET /readyz`

## Configuration
All config via environment variables (12-factor):
| Variable | Description |
|---|---|
| `DATABASE_URL` | Postgres DSN |
| `VALKEY_ADDR` | Valkey address |
| `OPENSEARCH_URL` | OpenSearch endpoint |
| `EVENTBRIDGE_BUS` | EventBridge bus name |
| `CACHE_TTL_SECONDS` | Product cache TTL |
| `BLOOM_REFRESH_SECONDS` | Bloom filter refresh interval |
| `PORT` | HTTP listen port (default 8080) |
| `INDEX_CHANNEL` | Target channel for indexer task (e.g. `web`) |
| `INDEX_LOCALE` | Target locale for indexer task (e.g. `en-GB`) |
| `S3_URI` | Snapshot S3 URI for importer task |

## Consequences
- No always-on indexer process; indexer only runs when triggered — lower cost, simpler ops
- EventBridge rule fan-out means adding a new channel or locale requires a new rule and no code change
- ECS Task cold start adds ~5–10s to the trigger-to-index latency; acceptable for a batch import pattern
- Distroless images reduce attack surface; use ECS Exec for debugging

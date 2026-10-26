# Infrastructure Diagram

> All diagrams are Mermaid. See `.kiro/steering/diagramming.md` for the diagramming standard.

## System Overview

```mermaid
graph TD
    Client["Clients / API Gateway"]
    Athena["AWS Athena\n(scheduled query)"]
    S3[("S3\ns3://bucket/catalog/snapshots/\n{dumped_at}/products.parquet")]
    EB_S3["EventBridge\nS3 rule"]
    EB_BUS["EventBridge\nbus: product-catalog"]

    subgraph Importer ["Importer (ECS Task)"]
        S3Reader["S3 File Reader"]
        SnapshotGuard["Snapshot Guard"]
    end

    subgraph Indexers ["Indexer Tasks (ECS, one per channel×locale)"]
        IDX_WEB_EN["indexer\nCHANNEL=web LOCALE=en-GB"]
        IDX_WEB_DE["indexer\nCHANNEL=web LOCALE=de-DE"]
        IDX_POS_EN["indexer\nCHANNEL=pos LOCALE=en-GB"]
        IDX_DOT["... one per combination"]
    end

    subgraph API ["API Service (ECS Service)"]
        Handler["HTTP Handlers"]
        Bloom["Bloom Filter (in-process)"]
        OutboxWorker["Outbox Worker (goroutine)"]
    end

    PG[("PostgreSQL\nproducts · variants · assets\nlocales · translations\nchannel_products · stock_levels\nsnapshots · outbox")]
    Valkey[("Valkey\nproduct:{id} TTL 5m")]
    OS[("OpenSearch\nproducts-web-en-gb\nproducts-web-de-de\nproducts-pos-en-gb · ...")]

    Athena -->|"UNLOAD to Parquet"| S3
    S3 -->|"s3:ObjectCreated"| EB_S3
    EB_S3 -->|"ECS RunTask"| Importer
    S3Reader --> SnapshotGuard
    SnapshotGuard -->|"UPSERT products/variants\n(snapshot guard)"| PG
    SnapshotGuard -->|"write outbox\nsnapshot.imported"| PG

    OutboxWorker -->|"poll outbox"| PG
    OutboxWorker -->|"PutEvents"| EB_BUS

    EB_BUS -->|"rule per channel×locale\nECS RunTask"| Indexers

    IDX_WEB_EN -->|"batch read\n(eligible products)"| PG
    IDX_WEB_EN -->|"bulk write\nalias swap"| OS
    IDX_WEB_DE -->|"batch read"| PG
    IDX_WEB_DE -->|"bulk write\nalias swap"| OS
    IDX_POS_EN -->|"batch read"| PG
    IDX_POS_EN -->|"bulk write\nalias swap"| OS

    Client -->|HTTPS| Handler
    Handler -->|"POST/PUT/PATCH/DELETE\n(write + outbox row)"| PG
    Handler -->|"GET /{id}\nbloom check"| Bloom
    Bloom -->|"may exist → cache lookup"| Valkey
    Valkey -->|"cache miss"| PG
    Handler -->|"GET /search\nchannel+locale → alias"| OS
    Handler -.->|"invalidate on write"| Valkey
```

## Single-Product GET Flow

```mermaid
sequenceDiagram
    participant C as Client
    participant A as API Service
    participant BF as Bloom Filter
    participant V as Valkey
    participant PG as PostgreSQL

    C->>A: GET /catalog/product/v1/{id}
    A->>BF: check(id)
    alt ID definitely absent
        BF-->>A: false
        A-->>C: 404 Not Found
    else ID may exist
        BF-->>A: true
        A->>V: GET product:{id}
        alt cache hit
            V-->>A: cached JSON
            A-->>C: 200 + ETag
        else cache miss
            V-->>A: nil
            A->>PG: SELECT product WHERE id=$1
            alt not found
                PG-->>A: no rows
                A-->>C: 404 Not Found
            else found
                PG-->>A: product row
                A->>V: SET product:{id} TTL 5m
                A-->>C: 200 + ETag
            end
        end
    end
```

## Mutation Flow (Optimistic Locking + Outbox)

```mermaid
sequenceDiagram
    participant C as Client
    participant A as API Service
    participant PG as PostgreSQL
    participant V as Valkey
    participant OW as Outbox Worker
    participant EB as EventBridge

    C->>A: PUT /catalog/product/v1/{id}\nIf-Match: "<etag>"
    A->>PG: BEGIN\nUPDATE products SET ..., version=4\nWHERE id=$1 AND version=3\nINSERT INTO outbox (event_type, payload)\nCOMMIT
    alt version mismatch (rows_affected=0)
        PG-->>A: 0 rows updated
        A-->>C: 412 Precondition Failed
    else success
        PG-->>A: 1 row updated
        A->>V: DEL product:{id}
        A-->>C: 202 ETag: "<new-etag>"
        OW->>PG: poll outbox WHERE dispatched_at IS NULL
        OW->>EB: PutEvents product.updated
        OW->>PG: UPDATE outbox SET dispatched_at=now()
    end
```

## Import Pipeline — Athena Snapshot Ingestion

```mermaid
sequenceDiagram
    participant ATH as Athena (scheduled)
    participant S3 as S3
    participant SQS as SQS (importer)
    participant IMP as Importer Job
    participant PG as PostgreSQL
    participant EB as EventBridge

    ATH->>S3: UNLOAD products TO\ns3://bucket/catalog/snapshots/{dumped_at}/
    S3->>SQS: s3:ObjectCreated notification
    SQS->>IMP: receive message (s3_uri, dumped_at)

    IMP->>PG: INSERT INTO snapshots (s3_uri, dumped_at)\nstatus='processing'

    loop for each product row in file
        IMP->>PG: INSERT INTO products ... ON CONFLICT (id) DO UPDATE\n  SET snapshot_id=$new, ...\n  WHERE snapshots.dumped_at > products→snapshot.dumped_at
        IMP->>PG: same guard for variants
    end

    IMP->>PG: UPDATE snapshots SET status='completed', row_count=$n
    IMP->>PG: INSERT INTO outbox (event_type='snapshot.imported')
    PG-->>EB: outbox worker publishes event
    EB-->>SQS: fan-out to indexer queue
```

> The snapshot guard (`WHERE snapshots.dumped_at > products→snapshot.dumped_at`) is evaluated in a single `UPDATE ... FROM snapshots` join, making it atomic per row. A re-run of the same dump is a no-op because `dumped_at` is equal, not greater.

## Indexer — Sequential Blue/Green Re-index per Channel×Locale

```mermaid
sequenceDiagram
    participant EB as EventBridge
    participant ECS as ECS (RunTask)
    participant IDX as Indexer Task\n(CHANNEL=web, LOCALE=en-GB)
    participant PG as PostgreSQL
    participant OS as OpenSearch

    EB->>ECS: snapshot.imported event\n→ rule fires per channel×locale
    ECS->>IDX: RunTask\nINDEX_CHANNEL=web\nINDEX_LOCALE=en-GB

    IDX->>OS: CREATE index products-web-en-gb-{timestamp}

    loop batches of 500 (keyset cursor)
        IDX->>PG: SELECT eligible products\n(published, stock!=OOS, channel enabled)\nwith en-GB translations + web price overrides
        IDX->>OS: POST _bulk (500 docs)
    end

    IDX->>OS: POST _aliases\n  remove: products-web-en-gb → old\n  add:    products-web-en-gb → new
    IDX->>OS: DELETE old index
    IDX-->>ECS: exit 0
```

> Each channel×locale task runs concurrently — they write to different indexes so there is no contention. A single sequential worker handles hundreds of thousands of products in 1–2 minutes; parallelising within one index would cause OpenSearch merge pressure and be slower overall. See ADR-010.

## Observability Pipeline

```mermaid
graph LR
    subgraph Services
        API2["API Service"]
        IDX2["Indexer Job"]
    end

    subgraph Collector ["OTel Collector"]
        RECV["OTLP Receiver\n:4317 gRPC"]
    end

    subgraph Local ["Local (docker-compose)"]
        Jaeger["Jaeger :16686\n(traces)"]
        Prom["Prometheus :9090\n(metrics)"]
    end

    subgraph Prod ["Production (AWS)"]
        XRay["AWS X-Ray\n(traces)"]
        AMP["Amazon Managed\nPrometheus (metrics)"]
        CW["CloudWatch Logs\n(logs)"]
    end

    API2 -->|"OTLP/gRPC"| RECV
    IDX2 -->|"OTLP/gRPC"| RECV
    API2 -->|"/metrics scrape"| Prom

    RECV --> Jaeger
    RECV --> Prom
    RECV --> XRay
    RECV --> AMP
    RECV --> CW
```

## Local Development Services (docker-compose)

| Service | Port | Purpose |
|---|---|---|
| `postgres` | 5432 | Primary data store |
| `valkey` | 6379 | Cache + idempotency keys |
| `opensearch` | 9200 | Search index |
| `localstack` | 4566 | SQS + EventBridge (local AWS) |
| `otel-collector` | 4317 | OTLP gRPC receiver |
| `jaeger` | 16686 | Trace UI |
| `prometheus` | 9090 | Metrics UI |
| `api` | 8080 | API service |
| `indexer` | — | Runs once on start for local dev (CHANNEL+LOCALE from env); in prod triggered by EventBridge |

## Key Design Decisions

| ADR | Decision |
|---|---|
| ADR-001 | Go as the implementation language |
| ADR-002 | OpenSearch for search and list |
| ADR-003 | PostgreSQL over a document database |
| ADR-004 | Valkey cache + Bloom filter for GET by ID |
| ADR-005 | Outbox pattern + EventBridge fan-out |
| ADR-006 | Idempotency keys + optimistic locking (If-Match / ETag) |
| ADR-007 | Kubernetes / ECS deployment |
| ADR-008 | OpenTelemetry for tracing, metrics, and logging |
| ADR-010 | Indexer design — sequential worker, per-channel-locale indexes, ECS Task trigger |

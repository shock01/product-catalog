# Infrastructure Diagram

> All diagrams are Mermaid. See `.kiro/steering/diagramming.md` for the diagramming standard.

## System Overview

```mermaid
graph TD
    Client["Clients / API Gateway"]
    Athena["AWS Athena\n(scheduled query)"]
    S3[("S3\ns3://bucket/catalog/snapshots/\n{dumped_at}/products.parquet")]
    S3P[("S3\ns3://bucket/pricing/")]
    KafkaP["External Kafka\npricing.price-changed"]
    KafkaI["External Kafka\ninventory.stock-changed"]
    EB_S3["EventBridge\nS3 rule (catalog)"]
    EB_S3P["EventBridge\nS3 rule (pricing)"]
    EB_BUS["EventBridge\nbus: product-catalog"]

    subgraph Importer ["Importer (ECS Task)"]
        S3Reader["S3 File Reader"]
        SnapshotGuard["Snapshot Guard"]
    end

    subgraph PriceConsumer ["price-consumer (ECS Service)"]
        KPConsumer["Kafka Consumer"]
        Batcher["S3 Batch Writer"]
    end

    subgraph PriceImporter ["price-importer (ECS Task)"]
        S3PReader["S3 JSON Reader"]
        PriceGuard["Price Snapshot Guard"]
    end

    subgraph InventoryConsumer ["inventory-consumer (ECS Service)"]
        KIConsumer["Kafka Consumer"]
        StockUpsert["Stock Upsert\n(last-write-wins)\n+ outbox row"]
    end

    subgraph Indexers ["Indexer Tasks (ECS, one per channel x locale)"]
        IDX_WEB_EN["indexer\nCHANNEL=web LOCALE=en-GB"]
        IDX_WEB_DE["indexer\nCHANNEL=web LOCALE=de-DE"]
        IDX_POS_EN["indexer\nCHANNEL=pos LOCALE=en-GB"]
        IDX_DOT["... one per combination"]
    end

    subgraph API ["API Service (ECS Service)"]
        Handler["HTTP Handlers"]
        Bloom["Bloom Filter (in-process)"]
        OutboxWorker["Outbox Worker (goroutine)"]
    end

    PG[("PostgreSQL\nproducts · variants · assets\nlocales · translations\nchannel_products · stock_levels\nsnapshots · outbox\nprice_snapshots · price_snapshot_imports\nv_current_prices · v_products_current")]
    Valkey[("Valkey\nproduct:{id} TTL 5m")]
    OS[("OpenSearch\nproducts-web-en-gb\nproducts-web-de-de\nproducts-pos-en-gb · ...")]

    Athena -->|"UNLOAD to Parquet"| S3
    S3 -->|"s3:ObjectCreated"| EB_S3
    EB_S3 -->|"ECS RunTask"| Importer
    S3Reader --> SnapshotGuard
    SnapshotGuard -->|"UPSERT products/variants\n(snapshot guard)"| PG
    SnapshotGuard -->|"write outbox\nsnapshot.imported"| PG

    KafkaP -->|"consume"| KPConsumer
    KPConsumer --> Batcher
    Batcher -->|"JSON lines batch\n(5 min or 1k records)"| S3P
    Athena -->|"UNLOAD prices to JSON"| S3P
    S3P -->|"s3:ObjectCreated"| EB_S3P
    EB_S3P -->|"ECS RunTask"| PriceImporter
    S3PReader --> PriceGuard
    PriceGuard -->|"INSERT price_snapshots\n(effective_from guard)"| PG
    PriceGuard -->|"write outbox\nprice.imported"| PG

    KafkaI -->|"consume"| KIConsumer
    KIConsumer --> StockUpsert
    StockUpsert -->|"UPSERT stock_levels\n+ outbox row\n(single transaction)"| PG

    OutboxWorker -->|"poll outbox"| PG
    OutboxWorker -->|"PutEvents"| EB_BUS

    EB_BUS -->|"snapshot.imported or price.imported\nrule per channel x locale -> full re-index\nECS RunTask"| Indexers
    EB_BUS -->|"stock.updated\npartial _update\nECS RunTask"| Indexers

    IDX_WEB_EN -->|"batch read\n(v_products_current\n+ v_current_prices)"| PG
    IDX_WEB_EN -->|"bulk write or partial _update\nalias swap on full re-index"| OS
    IDX_WEB_DE -->|"batch read"| PG
    IDX_WEB_DE -->|"bulk write or partial _update"| OS
    IDX_POS_EN -->|"batch read"| PG
    IDX_POS_EN -->|"bulk write or partial _update"| OS

    Client -->|HTTPS| Handler
    Handler -->|"POST/PUT/PATCH/DELETE\n(write + outbox row)"| PG
    Handler -->|"GET /{id}\nbloom check"| Bloom
    Bloom -->|"may exist -> cache lookup"| Valkey
    Valkey -->|"cache miss"| PG
    Handler -->|"GET /search\nchannel+locale -> alias"| OS
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
        IMP->>PG: INSERT INTO products ... ON CONFLICT (id) DO UPDATE\n  SET snapshot_id=$new, ...\n  WHERE snapshots.dumped_at > products->snapshot.dumped_at
        IMP->>PG: same guard for variants
    end

    IMP->>PG: UPDATE snapshots SET status='completed', row_count=$n
    IMP->>PG: INSERT INTO outbox (event_type='snapshot.imported')
    PG-->>EB: outbox worker publishes event
    EB-->>SQS: fan-out to indexer queue
```

> The snapshot guard (`WHERE snapshots.dumped_at > products->snapshot.dumped_at`) is evaluated in a single `UPDATE ... FROM snapshots` join, making it atomic per row. A re-run of the same dump is a no-op because `dumped_at` is equal, not greater.

## Pricing Pipeline

```mermaid
sequenceDiagram
    participant K as External Kafka<br/>pricing.price-changed
    participant PC as price-consumer<br/>(ECS Service)
    participant S3 as S3
    participant EB as EventBridge
    participant PI as price-importer<br/>(ECS Task)
    participant PG as PostgreSQL
    participant OW as Outbox Worker
    participant IDX as Indexer Tasks

    note over K,PC: Path A — stream (near-real-time)
    K->>PC: consume records
    PC->>PC: accumulate batch<br/>(5 min or 1,000 records)
    PC->>S3: write JSON lines<br/>pricing/stream/{date}/{hour}/{uuid}.json
    PC->>K: commit offsets (after S3 write)

    note over S3,IDX: Path B — Athena full import feeds same path from S3
    S3->>EB: s3:ObjectCreated
    EB->>PI: ECS RunTask<br/>S3_BUCKET, S3_KEY

    PI->>PG: INSERT price_snapshot_imports<br/>status='processing'
    loop for each price record
        PI->>PG: INSERT INTO price_snapshots<br/>WHERE no newer effective_from exists<br/>for product x channel x marketplace
    end
    PI->>PG: UPDATE price_snapshot_imports<br/>status='completed'
    PI->>PG: INSERT INTO outbox<br/>event_type='price.imported'

    OW->>PG: poll outbox
    OW->>EB: PutEvents price.imported
    EB->>IDX: rule per channel x locale<br/>ECS RunTask (full re-index)
    IDX->>PG: batch read via<br/>v_products_current + v_current_prices
    IDX->>IDX: bulk write + alias swap
```

> The Kafka consumer (price-consumer) is the only always-on component in the pricing pipeline. It commits Kafka offsets only after the S3 write succeeds, ensuring at-least-once delivery. The price-importer's `effective_from` guard makes re-processing idempotent.

## Inventory Pipeline

```mermaid
sequenceDiagram
    participant K as External Kafka<br/>inventory.stock-changed
    participant IC as inventory-consumer<br/>(ECS Service)
    participant PG as PostgreSQL
    participant OW as Outbox Worker
    participant EB as EventBridge
    participant IDX as Indexer Tasks
    participant OS as OpenSearch

    K->>IC: consume record<br/>{variant_id, warehouse_code, level}
    IC->>PG: BEGIN\nUPSERT stock_levels (IS DISTINCT FROM guard)\nINSERT INTO outbox (stock.updated) -- only if level changed\nCOMMIT
    IC->>K: commit Kafka offset

    OW->>PG: poll outbox
    OW->>EB: PutEvents stock.updated
    EB->>IDX: rule per channel x locale<br/>ECS RunTask (partial update)

    IDX->>PG: lookup product_id for variant\nrecompute aggregate stock status
    alt product transitions to fully OOS
        IDX->>OS: DELETE /{index}/_doc/{product_id}
    else stock fields changed
        IDX->>OS: POST /{index}/_update/{product_id}\n{stock fields only}
    end
```

> No S3 landing for inventory — the `IS DISTINCT FROM` guard makes the upsert idempotent, so at-least-once Kafka delivery is safe without a replay buffer. Stock changes trigger a partial OpenSearch `_update` rather than a full re-index, keeping propagation fast (seconds, not minutes).

## Indexer — Sequential Blue/Green Re-index per Channel x Locale

```mermaid
sequenceDiagram
    participant EB as EventBridge
    participant ECS as ECS (RunTask)
    participant IDX as Indexer Task\n(CHANNEL=web, LOCALE=en-GB)
    participant PG as PostgreSQL
    participant OS as OpenSearch

    EB->>ECS: snapshot.imported or price.imported\n-> rule fires per channel x locale
    ECS->>IDX: RunTask\nINDEX_CHANNEL=web\nINDEX_LOCALE=en-GB

    IDX->>OS: CREATE index products-web-en-gb-{timestamp}

    loop batches of 500 (keyset cursor)
        IDX->>PG: SELECT eligible products\n(published, stock!=OOS, channel enabled)\nvia v_products_current + v_current_prices
        IDX->>OS: POST _bulk (500 docs)
    end

    IDX->>OS: POST _aliases\n  remove: products-web-en-gb -> old\n  add:    products-web-en-gb -> new
    IDX->>OS: DELETE old index
    IDX-->>ECS: exit 0
```

> Each channel x locale task runs concurrently — they write to different indexes so there is no contention. A single sequential worker handles hundreds of thousands of products in 1–2 minutes; parallelising within one index would cause OpenSearch merge pressure and be slower overall. See ADR-010.

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
| ADR-011 | Pricing pipeline — Kafka consumer, S3 landing, price-importer, append-only snapshots |
| ADR-012 | Inventory pipeline — direct Postgres write, outbox, partial OpenSearch update |

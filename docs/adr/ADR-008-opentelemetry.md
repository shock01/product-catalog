# ADR-008: OpenTelemetry for Tracing, Metrics, and Logging

## Status
Accepted

## Context
The catalog service spans multiple components (API, indexer, outbox worker, Postgres, Valkey, OpenSearch, SQS). Debugging latency issues and errors across these boundaries requires correlated traces, consistent metrics, and structured logs that can be queried together.

## Decision
Use the OpenTelemetry (OTel) SDK as the single observability instrumentation layer for traces, metrics, and logs. Export via the OTel Collector to backend(s) of choice.

## Signal Design

### Traces
- Instrument with `go.opentelemetry.io/otel` and `go.opentelemetry.io/contrib/instrumentation`
- Auto-instrument: `net/http` (incoming requests), `pgx` (DB spans), `go-redis` (Valkey spans), `opensearch-go` (search spans), `aws-sdk-go-v2` (SQS/EventBridge spans)
- Propagate context via `traceparent` / W3C Trace Context headers on all inbound and outbound HTTP calls
- Every outbox event carries `trace_id` + `span_id` in its payload so downstream consumers can link spans across service boundaries

### Metrics
- Expose via OTel SDK → Prometheus exporter (scraped at `/metrics`) **and** OTLP push to the collector
- Key metrics:
  - `http_request_duration_seconds` (histogram, labelled by method + route + status)
  - `bloom_filter_hits_total` / `bloom_filter_misses_total`
  - `cache_hits_total` / `cache_misses_total` (labelled by `store=valkey`)
  - `outbox_pending_total` (gauge — rows not yet dispatched)
  - `outbox_dispatch_duration_seconds` (histogram)
  - `index_job_duration_seconds` (histogram, labelled by `mode=full|incremental`)
  - `opensearch_index_doc_count` (gauge)

### Logs
- Use `log/slog` with a custom OTel handler that attaches `trace_id` and `span_id` to every log record
- Log level controlled via `LOG_LEVEL` env var (default `info`)
- All logs are structured JSON; no free-form string concatenation

## Collector
- In local development: `otel/opentelemetry-collector-contrib` container in docker-compose, exporting to Jaeger (traces) and Prometheus (metrics)
- In production (ECS/K8s): OTel Collector runs as a sidecar (ECS) or DaemonSet (K8s), forwarding to AWS X-Ray (traces) and Amazon Managed Prometheus / CloudWatch (metrics)

## Rationale
- Single SDK for all three signals avoids maintaining separate libraries (e.g. separate Prometheus client + Jaeger client + logrus)
- Vendor-neutral: swap backends (Jaeger → Tempo, Prometheus → AMP) by changing collector config, not application code
- Trace-log correlation (injecting trace/span IDs into log records) is essential for debugging distributed failures across API → outbox → indexer
- W3C Trace Context propagation through EventBridge event payloads enables end-to-end traces across async boundaries

## Consequences
- OTel Go SDK adds ~5ms to startup time and a small per-span allocation overhead — negligible at this scale
- Collector is an additional infrastructure component to operate; in ECS/K8s this is a well-understood pattern
- Teams must adopt structured logging discipline — no `fmt.Println` in production paths

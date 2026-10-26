# ADR-001: Go as the Implementation Language

## Status
Accepted

## Context
We need to choose a backend language for the product catalog service. The service must handle high read throughput (reads vastly outnumber writes), low-latency single-product lookups, and background jobs (indexer, outbox worker).

## Decision
Use Go (Golang).

## Rationale
- Compiled binary with low memory footprint — well-suited for containerised deployment on ECS/Kubernetes
- Excellent standard library for HTTP servers (`net/http`) and concurrency primitives (goroutines, channels) needed for the outbox poller and indexer
- Strong ecosystem for our chosen infrastructure: `pgx` (Postgres), `go-redis` (Valkey), `opensearch-go`, `aws-sdk-go-v2`
- Fast cold-start times benefit ECS Fargate and Kubernetes rolling deploys
- Static typing catches contract mismatches between layers at compile time

## Consequences
- Team must be comfortable with Go idioms (interfaces, error wrapping, context propagation)
- No runtime reflection-heavy ORMs; SQL is written explicitly via `pgx` — more verbose but more transparent

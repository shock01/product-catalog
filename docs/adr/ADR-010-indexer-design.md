# ADR-010: Indexer Design — Sequential Worker, Per-Channel-Locale Indexes

## Status
Accepted

## Context
We need to index a few hundred thousand products into OpenSearch after each Athena snapshot import. The question is: how should the indexer be structured — parallel workers consuming a queue, or a simpler sequential approach? And should there be one shared index or separate indexes per channel and locale?

## Decision
1. **One index per channel × locale**, named `products-{channel}-{locale}-{timestamp}`
2. **One sequential Go worker per index** — no internal queue, no parallelism within a single index
3. **One ECS Task per channel × locale**, triggered by EventBridge on `snapshot.imported`

## Rationale

### Why sequential, not parallel?

A few hundred thousand products is not a big-data problem for a single process. A Go worker that:
- reads Postgres in batches of 500 (a simple `LIMIT`/`OFFSET` or keyset cursor)
- builds denormalised OpenSearch documents in memory
- sends bulk requests of 500 documents

...will sustain ~5,000–10,000 docs/sec against a reasonably sized OpenSearch cluster. At that rate, 500k products takes 1–2 minutes. That is fast enough for a batch import pattern.

**Parallelising writers to the same index is actively harmful:**
- Concurrent bulk requests to the same index compete for the same primary shards
- OpenSearch's indexing buffer fills faster, triggering more frequent flushes and segment merges
- Merge storms cause write throttling (`429 Too Many Requests`), which requires backoff/retry logic and ultimately makes the job slower and more complex
- The operational overhead of a queue (SQS, visibility timeouts, dead-letter queues, poison-pill handling) is not justified when a single worker already meets the latency requirement

### Why per-channel × locale?

Each channel and locale combination requires a different projection of the same product:
- **Channel**: different price overrides, different `enabled` flags, different eligibility
- **Locale**: different title, slug, description, option value labels

Resolving these at query time requires runtime joins or script fields — expensive at scale. Resolving them at index time produces a flat, fast document with no runtime computation.

Separate indexes also mean:
- A re-index of `web/en-GB` does not block or affect `pos/de-DE`
- Index mappings can be tuned per channel (e.g. different analysers per locale)
- A channel or locale can be taken offline by simply not pointing an alias at a new index

### Why ECS Tasks, not a long-running consumer?

The indexer only needs to run when there is new data. An always-on SQS consumer wastes compute and adds operational complexity (health checks, restarts, visibility timeout tuning). An ECS Task triggered by EventBridge:
- starts when needed, exits when done
- is stateless and idempotent (re-running produces the same index)
- scales naturally: adding a new channel or locale means adding one EventBridge rule, no code change

## Index Naming and Alias Convention

```
Index:  products-{channel}-{locale}-{yyyyMMddHHmmss}
Alias:  products-{channel}-{locale}

Example:
  products-web-en-gb-20240115120000  ← live index
  products-web-en-gb                 ← alias (what the API queries)
```

The indexer job receives `INDEX_CHANNEL` and `INDEX_LOCALE` as environment variables. It:
1. Creates `products-{channel}-{locale}-{timestamp}`
2. Bulk-loads all eligible products for that channel+locale
3. Atomically swaps the alias
4. Deletes the previous index

A failed job leaves the alias pointing at the previous index — zero downtime, safe fallback.

## Trigger Flow

```
snapshot.imported event on EventBridge
  → EventBridge rule: for each (channel, locale) pair
    → ECS RunTask: indexer
        env: INDEX_CHANNEL=web, INDEX_LOCALE=en-GB
        env: INDEX_CHANNEL=web, INDEX_LOCALE=de-DE
        env: INDEX_CHANNEL=pos, INDEX_LOCALE=en-GB
        ...
```

Tasks run concurrently across channel×locale combinations. Each writes to a different index — no contention, no coordination needed.

## Consequences
- Index count = `channels × locales` (e.g. 3 × 5 = 15 indexes). Manageable; OpenSearch handles hundreds of indexes comfortably
- No SQS queue for the indexer — simpler ops, fewer moving parts
- Re-index latency is minutes, not seconds — acceptable for a batch import pattern driven by Athena snapshots
- Adding a channel or locale requires a new EventBridge rule and a new alias; no code change
- The indexer binary is a simple `main()` that exits with code 0 on success, non-zero on failure — ECS handles retries via task restart policy

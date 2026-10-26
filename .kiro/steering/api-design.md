# API Design

## Contract-First
- The OpenAPI 3.1 spec at `api/openapi.yaml` is the single source of truth — see `.kiro/steering/openapi.md` for location rationale, generation tooling, and workflow rules
- Never modify generated files by hand

## Input Validation
All request bodies and query parameters are validated against the OpenAPI schema **before** they reach the application layer. Validation is enforced at the HTTP handler boundary using the generated types from `oapi-codegen`.

### Rules encoded in the spec
- `required` fields must be present — missing required fields return `400`
- `minLength` / `maxLength` on all string fields — prevents empty strings and oversized payloads
- `pattern` on machine-identifier fields (`code` on Option, OptionValue; `sku` on Variant; `locale`, `currency`) — enforces format without ambiguity
- `minimum` on numeric fields (`price`, `sort_order`) — price must be > 0; sort_order >= 0
- `minItems` on required arrays (`variants`, `translations`, option `values`) — already in spec; enforced at handler
- `format: uri` on asset URLs — validated by the JSON schema `uri` format checker
- `format: uuid` on all ID fields

### What validation does NOT cover
Schema validation catches structural and format errors (`400 Bad Request`). Domain invariant violations — e.g. a variant referencing an option value code not declared in `options` — are caught by `Product.Validate()` in the domain layer and returned as `422 Unprocessable Entity`.

### Error response
Validation failures return `400 Bad Request` with the standard error shape:
```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "variants[0].sku: does not match pattern '^[A-Z0-9][A-Z0-9-]{1,63}$'"
  }
}
```

## Command Query Separation (CQS)
See `.kiro/steering/cqs.md` for the full principle, layer-by-layer rules, and rationale.

- **Queries** (`GET`) return `200` with the full resource body (`data` + `meta` + `links`)
- **Commands** (`POST`, `PUT`, `PATCH`, `DELETE`) return `202 Accepted` with a `CommandResponse` containing only `links` — no resource body
- The client follows the `self` link to fetch updated state; the `ETag` on the `202` enables chained mutations without a round-trip GET

## Idempotency & Optimistic Locking
- All mutating endpoints (PUT, PATCH, DELETE) require an `If-Match: "<version>"` header
- The version value is the `ETag` returned on GET responses
- Returns `412 Precondition Failed` if the version does not match
- POST (create) accepts an optional `Idempotency-Key` header; duplicate requests with the same key return the original response

## Pagination
- List endpoints use cursor-based pagination: `?cursor=<opaque>&limit=<n>`
- Default limit: 20, max: 100
- Response includes `next_cursor` when more results exist

## Filtering (list/search)
- Query params: `category`, `min_price`, `max_price`, `locale`, `channel`, `q` (full-text)
- Filtering is served from OpenSearch; single-product GET is served from Valkey/Postgres

## Standard Response Shape
```json
{
  "data": { ... },
  "meta": { "version": "3", "cursor": "..." }
}
```

## Error Shape
```json
{
  "error": {
    "code": "PRODUCT_NOT_FOUND",
    "message": "product with id ... not found"
  }
}
```

## Key Endpoints
See `.kiro/steering/url-structure.md` for the full path convention, context split, and OAuth scope design.

Summary:
- `catalog/product/v1` — CRUD against PostgreSQL; requires `catalog:product:read` or `catalog:product:write`
- `catalog/search/v1` — search/list against OpenSearch; requires `catalog:search:read`
- `GET /healthz` — liveness probe
- `GET /readyz` — readiness probe

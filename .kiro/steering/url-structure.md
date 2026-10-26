# URL Structure & Context Design

## Path Convention

All paths follow the pattern:

```
/{domain}/{context}/{version}/{id}
```

| Segment | Description | Example |
|---|---|---|
| `domain` | The bounded domain this service owns | `catalog` |
| `context` | The sub-context within the domain | `product`, `search` |
| `version` | API version | `v1` |
| `id` | Aggregate root ID (UUID) — only on single-resource endpoints | `{id}` |

### Rules
- **Maximum path depth is `/{domain}/{context}/{version}/{id}`** — no additional segments beyond the aggregate ID
- Sub-resources (variants, assets, translations) are **never** exposed as nested path segments; they are part of the product aggregate and returned within the product representation or mutated via the product endpoint with a request body that identifies the sub-resource
- `{id}` is always the ID of the aggregate root (`Product`), never a child entity ID
- Path segments are lowercase, hyphen-separated if multi-word (e.g. `product-catalog` not `productCatalog`)

---

## Contexts

### `catalog/product` — backed by PostgreSQL (via Valkey cache)

Owns all mutations and single-resource reads for the Product aggregate.

| Method | Path | Description |
|---|---|---|
| `POST` | `/catalog/product/v1` | Create a product |
| `GET` | `/catalog/product/v1/{id}` | Get a single product by ID |
| `PUT` | `/catalog/product/v1/{id}` | Full update (requires `If-Match`) |
| `PATCH` | `/catalog/product/v1/{id}` | Partial update (requires `If-Match`) |
| `DELETE` | `/catalog/product/v1/{id}` | Soft delete (requires `If-Match`) |

### `catalog/search` — backed by OpenSearch

Read-only. Serves all list, filter, and full-text search operations.

| Method | Path | Description |
|---|---|---|
| `GET` | `/catalog/search/v1` | Search / list products with filters and pagination |

---

## OAuth Scopes

Contexts map directly to OAuth 2.0 scopes, allowing fine-grained access control per consumer.

| Scope | Grants access to | Typical consumer |
|---|---|---|
| `catalog:product:read` | `GET /catalog/product/v1/{id}` — any product status | PIM, admin tooling, internal services |
| `catalog:product:write` | `POST`, `PUT`, `PATCH`, `DELETE` on `catalog/product/v1` | PIM, admin tooling, import pipelines |
| *(none)* | `GET /catalog/search/v1` | Storefront, mobile app, public search UI |

### Rules
- `catalog/search` is a **public endpoint** — no OAuth scope required. Visibility is enforced at index time: only `published` products with at least one non-OOS variant and an enabled channel listing are indexed. There is nothing sensitive in the OpenSearch index.
- `catalog/product` is an **internal endpoint** — always requires a scope. It returns products of any status (including `draft`, `archived`). Public storefronts must not be granted `catalog:product:read`.
- Public storefronts discover products via `catalog/search` and follow the `self` link in search results. They never need to call `catalog/product/{id}` directly.
- Do not introduce a weaker `catalog:product:read:public` scope that changes the behaviour of `GET /catalog/product/v1/{id}` based on the token. That creates a hidden API version and breaks cache sharing. If visibility filtering by status is needed for a new consumer, introduce a new endpoint or context.
- Write scopes require `catalog:product:write`; read-only internal consumers should only be granted `catalog:product:read`.
- Scopes are declared in the OpenAPI spec under `securitySchemes` and applied per-operation via `security` blocks.
- The API gateway / auth middleware enforces scopes; the application layer trusts the validated token claims.

---

## OpenAPI Declaration

```yaml
# api/openapi.yaml (excerpt)

info:
  title: Product Catalog API
  version: 1.0.0

servers:
  - url: https://api.example.com

components:
  securitySchemes:
    oauth2:
      type: oauth2
      flows:
        clientCredentials:
          tokenUrl: https://auth.example.com/oauth/token
          scopes:
            catalog:product:read:  Read a single product by ID (any status). Internal consumers only.
            catalog:product:write: Create, update, and delete products. Internal consumers only.

paths:
  /catalog/product/v1:
    post:
      operationId: createProduct
      security:
        - oauth2: [catalog:product:write]

  /catalog/product/v1/{id}:
    get:
      operationId: getProduct
      security:
        - oauth2: [catalog:product:read]
    put:
      operationId: replaceProduct
      security:
        - oauth2: [catalog:product:write]
    patch:
      operationId: updateProduct
      security:
        - oauth2: [catalog:product:write]
    delete:
      operationId: deleteProduct
      security:
        - oauth2: [catalog:product:write]

  /catalog/search/v1:
    get:
      operationId: searchProducts
      security: []  # public — no scope required
```

---

## Rationale

### Why `/{domain}/{context}/{version}` instead of `/v1/{resource}`?

- **Context isolation**: `catalog/product` and `catalog/search` are backed by different datastores (RDS vs OpenSearch) and have different SLAs, scaling characteristics, and auth requirements. Separating them in the path makes this explicit and allows independent routing at the gateway layer.
- **Scope granularity**: A storefront only needs `catalog:search:read`. Granting it `catalog:product:read` would expose the RDS-backed endpoint unnecessarily. Path-aligned scopes make least-privilege straightforward.
- **Evolvability**: `catalog/product/v2` can be introduced alongside `v1` without affecting `catalog/search/v1`. Versioning is per-context, not global.
- **No nested sub-resources**: Keeping paths flat (max depth = ID) avoids the proliferation of `/products/{id}/variants/{variantId}/assets/{assetId}` style URLs that couple the API shape to the internal data model. The aggregate is the unit of API interaction.

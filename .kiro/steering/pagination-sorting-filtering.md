# Pagination, Sorting, and Filtering

## Pagination — limit/offset with Pages envelope

All list endpoints use **limit/offset pagination**, not cursor-based pagination.

### Query parameters

| Parameter | Type | Default | Max | Description |
|---|---|---|---|---|
| `limit` | integer | 20 | 100 | Number of results per page |
| `offset` | integer | 0 | — | Zero-based index of the first result |

### Pages envelope

Every paginated response includes a `pages` object in the response envelope:

```json
{
  "data": [...],
  "pages": {
    "total": 1842,
    "total_pages": 93,
    "page": 3,
    "page_size": 20
  }
}
```

| Field | Description |
|---|---|
| `total` | Total number of matching records/documents |
| `total_pages` | `ceil(total / page_size)` |
| `page` | Current page number (1-based: `floor(offset / limit) + 1`) |
| `page_size` | The effective `limit` used for this response |

### Rules
- `page` in the response is always 1-based, even though `offset` in the request is 0-based
- `total_pages` is 0 when `total` is 0
- Requesting beyond the last page returns an empty `data` array with the correct `pages` envelope — never a 404
- `offset` must be `>= 0`; `limit` must be between 1 and 100 inclusive

---

## Sorting

### Query parameters

| Parameter | Type | Description |
|---|---|---|
| `sort` | string | Field to sort by (see allowed values per endpoint) |
| `order` | string | `asc` or `desc`. Default: `asc` |

### Rules
- `sort` and `order` are always used together; `order` without `sort` is ignored
- Invalid `sort` field values return `400 Bad Request`
- `order` values other than `asc` / `desc` return `400 Bad Request`
- Default sort (when `sort` is omitted) is defined per endpoint and documented in the OpenAPI spec

### Allowed sort fields per endpoint

| Endpoint | Allowed `sort` values | Default |
|---|---|---|
| `GET /catalog/product/v1` | `created_at`, `updated_at`, `status` | `created_at` |
| `GET /catalog/search/v1` | `price`, `title`, `created_at` | `created_at` |
| `POST /catalog/search/v1/query` | `price`, `title`, `created_at` | `created_at` |

---

## Filtering

Filters are passed as query parameters on GET endpoints, or as a structured `filter` object in the request body on `POST /catalog/search/v1/query`.

### Rules
- Multiple values for the same filter are OR'd (e.g. `tag=sale&tag=new` matches products tagged `sale` OR `new`)
- Filters across different fields are AND'd (e.g. `brand=nike&tag=sale` matches Nike products that are also tagged `sale`)
- Unknown filter parameters are ignored (do not return 400) on GET endpoints
- The `POST /catalog/search/v1/query` body is strictly validated — unknown fields return `400`
- Price range filters (`min_price`, `max_price`) are inclusive on both ends
- String filters (brand, tag, taxonomy, category) are exact-match on the indexed code value, not full-text

---

## Search query parameter (`q`)

Used on `GET /catalog/search/v1` and `POST /catalog/search/v1/query`.

- Full-text search across `title`, `description`, and `short_description` fields in the requested locale
- Matched against the OpenSearch index for the requested `channel` × `locale` combination
- When `q` is absent, all eligible products are returned (filtered by other parameters)
- `q` is combined with filters using AND — results must match both the query and all filters

---

## Taxonomy path storage

Taxonomy paths are stored as `integer[]` on `product_taxonomies.path`, written explicitly at import/API write time. The full ancestor chain is always stored — e.g. `{1, 120, 780}` where `1` is the root and `780` is the leaf.

- The path is **never computed at query time** from the `taxonomies` tree — it is the responsibility of the importer and API handler to resolve and store the full chain
- The `taxonomies` table is used only to resolve human-readable labels (via `taxonomy_translations`) at index time and in the `v_products_current` view
- OpenSearch indexes the path array as a multi-value field, enabling prefix/containment filtering (e.g. filter on ancestor node `120` matches all products with `120` anywhere in their path)
- A product may belong to multiple taxonomy nodes (multiple rows in `product_taxonomies`)
- Taxonomy depth is 1–4 levels; `path` length must match `taxonomies.depth` for the leaf node

---

## Brand

- All products must have a `brand_id` — it is `NOT NULL` on `products`
- `brands` is a reference table synced from an external service (like `channels`, `marketplaces`)
- No display name or translations on `Brand` for now — `code` is the identifier used for filtering
- Brand filtering on search uses exact match on `brand_id` (UUID) or `brand.code` (string) depending on the index projection


A body-based search endpoint for clients that need more granular control than query parameters allow. Accepts the same parameters as `GET /catalog/search/v1` but as a structured JSON body, enabling:

- Multiple values per filter field (array syntax)
- Explicit boolean logic between filter groups (future extension)
- Cleaner client code for complex filter combinations

The response schema is identical to `GET /catalog/search/v1`.

```json
{
  "q": "classic t-shirt",
  "locale": "en-GB",
  "channel": "web",
  "limit": 20,
  "offset": 0,
  "sort": "price",
  "order": "asc",
  "filter": {
    "brand": ["nike", "adidas"],
    "tag": ["sale", "new"],
    "taxonomy": ["clothing/tops"],
    "category": ["tops"],
    "min_price": 10.00,
    "max_price": 50.00,
    "stock_status": "IN_STOCK"
  }
}
```

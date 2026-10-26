# Command Query Separation (CQS)

## Principle

Every operation is either a **query** or a **command** — never both.

| Type | Returns | Mutates state |
|---|---|---|
| **Query** | Data (the resource representation) | No |
| **Command** | Links only (`202 Accepted`) | Yes |

This is enforced at every layer: domain, application, and API.

---

## Application Layer (`internal/app`)

- **Query methods** return a value and an error: `func (s *Service) GetProduct(ctx, id) (*domain.Product, error)`
- **Command methods** return only an error: `func (s *Service) UpdateProduct(ctx, cmd) error`
- A command method must never return the mutated aggregate. If the caller needs the updated state it must issue a separate query.
- Command and query handlers live in separate files: `query.go` and `command.go` per use-case group.

## Domain Layer (`internal/domain`)

- Aggregate methods that mutate state return only `error` (or nothing).
- Aggregate methods that read state return a value.
- Domain events are produced as a side-effect of commands, never of queries.

---

## API Layer

Commands and queries map to distinct HTTP semantics:

### Queries → `200 OK` with full resource body

| Method | Endpoint | Returns |
|---|---|---|
| `GET` | `/catalog/product/v1/{id}` | Full `ProductResponse` (data + meta + links) |
| `GET` | `/catalog/search/v1` | `SearchResponse` (data + meta) |

### Commands → `202 Accepted` with links only

Mutating endpoints do **not** return the resource body. They return a `CommandResponse` containing only HATEOAS `links` so the client knows where to fetch the updated state.

| Method | Endpoint | Returns |
|---|---|---|
| `POST` | `/catalog/product/v1` | `202` + `CommandResponse` (links: self, search) |
| `PUT` | `/catalog/product/v1/{id}` | `202` + `CommandResponse` (links: self, search) |
| `PATCH` | `/catalog/product/v1/{id}` | `202` + `CommandResponse` (links: self, search) |
| `DELETE` | `/catalog/product/v1/{id}` | `202` + `CommandResponse` (links: search) |

The `Location` header on `POST` points to the newly created resource.
The `ETag` header on all command responses carries the new content-derived ETag so the client can issue the next mutation without a round-trip GET.

### Why `202` and not `200` / `201` / `204`?

`202 Accepted` signals that the command was received and will be processed. It makes the async nature of the system explicit — the outbox worker dispatches events after the HTTP response is returned. The client must not assume the downstream index or event consumers have processed the change by the time `202` is received.

---

## Error Responses for Commands

| Status | Meaning |
|---|---|
| `400 Bad Request` | Request body failed schema validation |
| `409 Conflict` | Duplicate SKU or idempotency key collision with a different payload |
| `412 Precondition Failed` | `If-Match` ETag does not match current resource version |
| `422 Unprocessable Entity` | Request is valid JSON but violates a domain invariant |

---

## Consequences

- Clients that need the updated resource after a mutation must issue a `GET` using the `self` link from the `CommandResponse`. This is an intentional extra round-trip — it keeps the write path simple and avoids re-reading inside a transaction.
- The `ETag` on the `202` response allows clients to chain mutations without a GET (they already have the new ETag for the next `If-Match`).

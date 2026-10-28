# Domain Services vs Application Services

## The distinction matters

Both are called "services" but they live in different layers and have different rules.

---

## Domain Services (`internal/domain`)

A domain service encapsulates **business logic that doesn't naturally belong to a single aggregate**.

Use a domain service when:
- The operation involves more than one aggregate or value object
- The logic is a pure business rule with no I/O
- The concept has a name in the domain language (e.g. "pricing", "eligibility")

Rules:
- Zero infrastructure imports — no `pgx`, no `redis`, no `http`, no `uuid` generation
- No I/O of any kind — no reading from or writing to anything external
- Inputs and outputs are domain types only
- Stateless — no fields that hold connections or clients

```go
// domain service — pure business logic, no I/O
type PricingService struct{}

func (s PricingService) EffectivePrice(v domain.Variant, ch domain.Channel) float64 {
    for _, cl := range v.ChannelListings {
        if cl.ChannelID == ch.ID && cl.PriceOverride != nil {
            return *cl.PriceOverride
        }
    }
    return v.Price
}
```

---

## Application Services (`internal/usecase`)

An application service **orchestrates** domain objects and infrastructure to fulfil a use-case.

`ProductService` in `internal/usecase` is an application service. It:
- Calls `domain.Product.Validate()` — delegates business rules to the domain
- Calls `repository.Save()` — delegates persistence to infrastructure
- Calls `cache.Del()` — manages the read-through cache
- Calls `outbox.Append()` — dispatches domain events
- Generates UUIDs and timestamps — application-level concerns

None of that is business logic. It is coordination. The domain doesn't know about Valkey, SHA-256 ETags, or outbox tables — and it shouldn't.

Rules:
- No HTTP types, no `net/http` imports
- No SQL — delegates to repository interfaces defined in `domain`
- Commands return only `error`; queries return a value and `error` (see `cqrs.md`)
- Owns transaction boundaries
- Wraps all errors from external interfaces with `fmt.Errorf("context: %w", err)`

---

## Decision rule

> Does this logic require knowing about a product's variants, options, or channel listings to produce a correct answer?
> → Domain service or aggregate method.

> Does this logic require reading from a database, writing to a cache, or dispatching an event?
> → Application service in `internal/usecase`.

---

## What `ProductService` is NOT

`ProductService` is not a domain service. It contains no business rules. If you find yourself writing an `if` statement in `ProductService` that checks a product field to decide what to do — that logic belongs in `domain.Product` or a domain service, not in the use-case.

The use-case should read like a script: fetch, validate, save, invalidate, dispatch. The intelligence is in the domain.

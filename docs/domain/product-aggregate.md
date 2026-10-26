# Product Aggregate

## Overview

`Product` is the aggregate root of the Catalog bounded context. It groups everything needed to represent a sellable item: its options, the variants those options produce, assets, translations, channel visibility, and stock levels.

The aggregate boundary is intentional: all mutations to variants, options, and channel listings go through the `Product` root. No child entity is mutated independently — the root enforces invariants and emits domain events.

---

## Aggregate Structure

```mermaid
graph TD
    PR["<b>Product</b> (Aggregate Root)\n─────────────────\nid: UUID\nsnapshot_id: UUID\nstatus: ProductStatus\nversion: int\ncreated_at / updated_at / deleted_at"]

    OPT["<b>Option</b>\n─────────────────\nid: UUID\ncode: string  e.g. 'size', 'colour'\nsort_order: int"]

    OT["<b>OptionTranslation</b>\n─────────────────\noption_id: UUID\nlocale: string\nname: string  e.g. 'Size', 'Größe'"]

    OV["<b>OptionValue</b>\n─────────────────\nid: UUID\noption_id: UUID\ncode: string  e.g. 'xl', 'red'\nsort_order: int"]

    OVT["<b>OptionValueTranslation</b>\n─────────────────\noption_value_id: UUID\nlocale: string\nvalue: string  e.g. 'XL', 'Rot'"]

    VAR["<b>Variant</b>\n─────────────────\nid: UUID\nsnapshot_id: UUID\nsku: string  ← identity of a purchasable unit\nprice: numeric\ncurrency: string\nversion: int"]

    VOV["<b>VariantOptionValue</b>\n─────────────────\nvariant_id: UUID\noption_value_id: UUID"]

    ASS["<b>Asset</b>\n─────────────────\nid: UUID\nurl: string\ntype: image|video\nsort_order: int\nscope: product|variant"]

    PT["<b>ProductTranslation</b>\n─────────────────\nlocale: string\ntitle: string\nslug: string\ndescription: string\nshort_description: string\nmeta_title: string\nmeta_description: string"]

    VT["<b>VariantTranslation</b>\n─────────────────\nlocale: string\ntitle: string\ndescription: string"]

    CP["<b>ChannelProduct</b>\n─────────────────\nchannel_id: UUID\nenabled: bool\nprice_override: numeric\ncurrency: string\nversion: int"]

    MP["<b>MarketplaceProduct</b>\n─────────────────\nmarketplace_id: UUID\nexternal_id: string\nfeed_attributes: jsonb\nversion: int"]

    SL["<b>StockLevel</b>\n─────────────────\nwarehouse_code: string\nlevel: StockStatus  IN_STOCK|LOW|OOS\nversion: int"]

    PR --> OPT
    OPT --> OT
    OPT --> OV
    OV --> OVT
    PR --> VAR
    VAR --> VOV
    VOV --> OV
    PR --> ASS
    VAR --> ASS
    PR --> PT
    VAR --> VT
    PR --> CP
    PR --> MP
    VAR --> SL
```

---

## Options and Variants

Options define the axes of variation for a product. Each option has a set of values. A variant is the concrete, purchasable combination of one value from each option.

```mermaid
graph LR
    subgraph Options
        O1["Option: Size\n  S · M · L · XL"]
        O2["Option: Colour\n  Red · Blue"]
    end

    subgraph Variants ["Variants (cartesian product)"]
        V1["SKU: SHIRT-S-RED\nSize=S, Colour=Red"]
        V2["SKU: SHIRT-S-BLU\nSize=S, Colour=Blue"]
        V3["SKU: SHIRT-M-RED\nSize=M, Colour=Red"]
        V4["SKU: SHIRT-M-BLU\nSize=M, Colour=Blue"]
        V5["SKU: SHIRT-L-RED\nSize=L, Colour=Red"]
        V6["..."]
    end

    O1 --> V1
    O1 --> V2
    O1 --> V3
    O1 --> V4
    O1 --> V5
    O2 --> V1
    O2 --> V2
    O2 --> V3
    O2 --> V4
    O2 --> V5
```

### Rules
- A product with **no options** has exactly **one implicit variant** (the product itself is the purchasable unit). The variant SKU is set explicitly.
- A product with options must have at least one variant. Not all combinations need to exist (e.g. XL is only available in Blue).
- The **SKU lives on the Variant**, never on the Product root. The Product has no SKU field.
- `Option.code` and `OptionValue.code` are locale-neutral machine identifiers (e.g. `size`, `xl`). Human-readable labels live exclusively in `OptionTranslation` and `OptionValueTranslation`.
- Deleting an option value that is still referenced by a variant is a domain invariant violation — the aggregate rejects it.

---

## Entity Responsibilities

| Entity | Owns | Does NOT own |
|---|---|---|
| `Product` | Status, aggregate version, channel/marketplace listings, `snapshot_id` | SKU, price, stock, any human-readable text |
| `Snapshot` | S3 URI, `dumped_at` timestamp, import status, row count | Product data — Snapshot is a write-guard, not a data owner |
| `Option` | Locale-neutral `code` (e.g. `size`) and sort order | Human-readable label — that lives in `OptionTranslation` |
| `OptionTranslation` | Localised option name per locale (e.g. "Size", "Größe") | — |
| `OptionValue` | Locale-neutral `code` (e.g. `xl`) and sort order | Human-readable label — that lives in `OptionValueTranslation` |
| `OptionValueTranslation` | Localised option value per locale (e.g. "XL", "Rot") | — |
| `Variant` | SKU, base price, currency, option value combination | Product-level metadata, human-readable text |
| `VariantOptionValue` | The join between a variant and its selected option values | — |
| `Asset` | URL, type, sort order; scoped to product or a specific variant | Storage / CDN |
| `ProductTranslation` | `title`, `slug`, `description`, `short_description`, `meta_title`, `meta_description` per locale | Variant-level copy |
| `VariantTranslation` | Localised `title` and `description` per locale | Option labels |
| `ChannelProduct` | Per-channel enabled flag and price override | Channel config — Channel is owned by the Channel Service |
| `MarketplaceProduct` | External ID and feed attributes per marketplace | Marketplace config — Marketplace is owned by the Marketplace Service |
| `StockLevel` | Stock status per variant per warehouse (`IN_STOCK`, `LOW`, `OOS`) | Exact quantity — that is owned by the Stock Service |

> `ChannelProduct.channel_id` and `MarketplaceProduct.marketplace_id` are references to **locally cached** reference data synced from external services. See ADR-009 § Foreign References for the sync strategy.

---

## Invariants

The aggregate root enforces these invariants on every mutation:

1. A `Published` product must have at least one enabled `ChannelProduct`.
2. Every `Variant` must reference exactly one `OptionValue` per `Option` defined on the product (or zero options → one implicit variant).
3. An `OptionValue` cannot be removed if any `Variant` still references it.
4. A `Variant` SKU must be unique within the aggregate (enforced at DB level with a unique index too).
5. `StockLevel.level` must be one of the valid `StockStatus` values: `IN_STOCK`, `LOW`, `OOS`.
6. All mutations increment `version` on the affected entity and on the `Product` root.

---

## Data Model (ER)

```mermaid
erDiagram
    PRODUCT {
        uuid id PK
        uuid snapshot_id FK
        string status
        int version
        timestamptz created_at
        timestamptz updated_at
        timestamptz deleted_at
    }
    OPTION {
        uuid id PK
        uuid product_id FK
        string code
        int sort_order
    }
    OPTION_TRANSLATION {
        uuid option_id FK
        string locale_code FK
        string name
    }
    OPTION_VALUE {
        uuid id PK
        uuid option_id FK
        string code
        int sort_order
    }
    OPTION_VALUE_TRANSLATION {
        uuid option_value_id FK
        string locale_code FK
        string value
    }
    VARIANT {
        uuid id PK
        uuid product_id FK
        uuid snapshot_id FK
        string sku
        numeric price
        string currency
        int version
    }
    VARIANT_OPTION_VALUE {
        uuid variant_id FK
        uuid option_value_id FK
    }
    ASSET {
        uuid id PK
        uuid product_id FK
        uuid variant_id FK
        string url
        string type
        int sort_order
    }
    PRODUCT_TRANSLATION {
        uuid product_id FK
        string locale_code FK
        string title
        string slug
        string description
        string short_description
        string meta_title
        string meta_description
    }
    VARIANT_TRANSLATION {
        uuid variant_id FK
        string locale_code FK
        string title
        string description
    }
    CHANNEL_PRODUCT {
        uuid channel_id FK
        uuid product_id FK
        bool enabled
        numeric price_override
        string currency
        int version
    }
    MARKETPLACE_PRODUCT {
        uuid marketplace_id FK
        uuid product_id FK
        string external_id
        jsonb feed_attributes
        int version
    }
    STOCK_LEVEL {
        uuid variant_id FK
        string warehouse_code
        string level
        int version
    }

    SNAPSHOT {
        uuid id PK
        string s3_uri
        timestamptz dumped_at
        timestamptz imported_at
        int row_count
        string status
    }

    PRODUCT ||--o{ OPTION : "defines"
    OPTION ||--o{ OPTION_TRANSLATION : "translated by"
    OPTION ||--o{ OPTION_VALUE : "has"
    OPTION_VALUE ||--o{ OPTION_VALUE_TRANSLATION : "translated by"
    PRODUCT ||--o{ VARIANT : "has"
    VARIANT ||--o{ VARIANT_OPTION_VALUE : "selected by"
    VARIANT_OPTION_VALUE }o--|| OPTION_VALUE : "references"
    PRODUCT ||--o{ ASSET : "has"
    VARIANT ||--o{ ASSET : "has"
    PRODUCT ||--o{ PRODUCT_TRANSLATION : "translated by"
    VARIANT ||--o{ VARIANT_TRANSLATION : "translated by"
    PRODUCT ||--o{ CHANNEL_PRODUCT : "listed in"
    PRODUCT ||--o{ MARKETPLACE_PRODUCT : "listed on"
    VARIANT ||--o{ STOCK_LEVEL : "stocked at"
    SNAPSHOT ||--o{ PRODUCT : "last imported by"
    SNAPSHOT ||--o{ VARIANT : "last imported by"
```

> `StockStatus` is a domain enum: `IN_STOCK` | `LOW` | `OOS` (out of stock). The Catalog never stores exact quantities — those are owned by the Stock Service. The Catalog receives a translated status via `stock.level_changed` events.

### Localisation coverage

| Entity | Localised fields |
|---|---|
| `ProductTranslation` | `title`, `slug`, `description`, `short_description`, `meta_title`, `meta_description` |
| `VariantTranslation` | `title`, `description` |
| `OptionTranslation` | `name` (e.g. "Size" → "Größe") |
| `OptionValueTranslation` | `value` (e.g. "Red" → "Rot") |

No human-readable text lives on `Product`, `Option`, `OptionValue`, or `Variant` directly. All display copy is locale-scoped.

---

## Example: T-Shirt Product

```
Product: "Classic T-Shirt"  (id: prod-001, status: published)
│
├── Option: "Size"  (sort: 0)
│   ├── OptionValue: "S"
│   ├── OptionValue: "M"
│   ├── OptionValue: "L"
│   └── OptionValue: "XL"
│
├── Option: "Colour"  (sort: 1)
│   ├── OptionValue: "Red"
│   └── OptionValue: "Blue"
│
├── Variant: SKU=TSHIRT-S-RED   → Size=S,  Colour=Red   price=19.99 GBP
├── Variant: SKU=TSHIRT-S-BLU   → Size=S,  Colour=Blue  price=19.99 GBP
├── Variant: SKU=TSHIRT-M-RED   → Size=M,  Colour=Red   price=19.99 GBP
├── Variant: SKU=TSHIRT-M-BLU   → Size=M,  Colour=Blue  price=19.99 GBP
├── Variant: SKU=TSHIRT-L-RED   → Size=L,  Colour=Red   price=19.99 GBP
├── Variant: SKU=TSHIRT-L-BLU   → Size=L,  Colour=Blue  price=19.99 GBP
├── Variant: SKU=TSHIRT-XL-BLU  → Size=XL, Colour=Blue  price=21.99 GBP  ← XL only in Blue
│
├── ProductTranslation: en-GB → title="Classic T-Shirt"
│                               slug="classic-t-shirt"
│                               description="A timeless wardrobe staple."
│                               short_description="Everyday comfort."
│                               meta_title="Classic T-Shirt | MyStore"
│                               meta_description="Shop the Classic T-Shirt..."
├── ProductTranslation: de-DE → title="Klassisches T-Shirt"
│                               slug="klassisches-t-shirt"
│                               description="Ein zeitloser Klassiker."
│
├── OptionTranslation (size):   en-GB="Size"    de-DE="Größe"
├── OptionValueTranslation (xl): en-GB="XL"     de-DE="XL"
├── OptionValueTranslation (red): en-GB="Red"   de-DE="Rot"
│
├── VariantTranslation TSHIRT-S-RED: en-GB → title="Classic T-Shirt — S / Red"
│
├── ChannelProduct: web  → enabled=true,  price_override=null
├── ChannelProduct: pos  → enabled=true,  price_override=17.99
│
└── Asset: url=cdn/tshirt-hero.jpg  scope=product
    Asset: url=cdn/tshirt-red-s.jpg scope=variant TSHIRT-S-RED
```

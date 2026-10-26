-- migrate:up

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ---------------------------------------------------------------------------
-- Reference tables (owned by external services, synced into Catalog)
-- ---------------------------------------------------------------------------

CREATE TABLE locales (
    code TEXT PRIMARY KEY  -- BCP-47, e.g. 'en-GB'
);

CREATE TABLE channels (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    code       TEXT        NOT NULL UNIQUE,
    name       TEXT        NOT NULL,
    active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE marketplaces (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    code       TEXT        NOT NULL UNIQUE,
    name       TEXT        NOT NULL,
    active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Import snapshots (one row per Athena dump file processed)
-- ---------------------------------------------------------------------------

CREATE TABLE snapshots (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    s3_uri      TEXT        NOT NULL UNIQUE,  -- e.g. s3://bucket/prefix/2024-01-15T12:00:00Z/
    dumped_at   TIMESTAMPTZ NOT NULL,          -- partition timestamp embedded in the S3 key
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    row_count   INTEGER,
    status      TEXT        NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending','processing','completed','failed'))
);

-- ---------------------------------------------------------------------------
-- Core catalog tables
-- ---------------------------------------------------------------------------

CREATE TABLE products (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    snapshot_id UUID        REFERENCES snapshots(id),  -- last snapshot that wrote this row
    status      TEXT        NOT NULL DEFAULT 'draft'
                            CHECK (status IN ('draft','active','published','archived')),
    version     INTEGER     NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE TABLE product_translations (
    product_id        UUID  NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    locale_code       TEXT  NOT NULL REFERENCES locales(code),
    title             TEXT  NOT NULL,
    slug              TEXT  NOT NULL,
    description       TEXT,
    short_description TEXT,
    meta_title        TEXT,
    meta_description  TEXT,
    PRIMARY KEY (product_id, locale_code)
);

CREATE UNIQUE INDEX uq_product_translations_slug_locale
    ON product_translations (locale_code, slug)
    WHERE slug IS NOT NULL;

-- ---------------------------------------------------------------------------
-- Options & values (locale-neutral codes + translations)
-- ---------------------------------------------------------------------------

CREATE TABLE options (
    id         UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID    NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    code       TEXT    NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    UNIQUE (product_id, code)
);

CREATE TABLE option_translations (
    option_id   UUID NOT NULL REFERENCES options(id) ON DELETE CASCADE,
    locale_code TEXT NOT NULL REFERENCES locales(code),
    name        TEXT NOT NULL,
    PRIMARY KEY (option_id, locale_code)
);

CREATE TABLE option_values (
    id         UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    option_id  UUID    NOT NULL REFERENCES options(id) ON DELETE CASCADE,
    code       TEXT    NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    UNIQUE (option_id, code)
);

CREATE TABLE option_value_translations (
    option_value_id UUID NOT NULL REFERENCES option_values(id) ON DELETE CASCADE,
    locale_code     TEXT NOT NULL REFERENCES locales(code),
    value           TEXT NOT NULL,
    PRIMARY KEY (option_value_id, locale_code)
);

-- ---------------------------------------------------------------------------
-- Variants
-- ---------------------------------------------------------------------------

CREATE TABLE variants (
    id          UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id  UUID           NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    snapshot_id UUID           REFERENCES snapshots(id),  -- last snapshot that wrote this row
    sku         TEXT           NOT NULL UNIQUE,
    price       NUMERIC(12, 4) NOT NULL,
    currency    TEXT           NOT NULL,  -- ISO 4217
    version     INTEGER        NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT now()
);

CREATE TABLE variant_option_values (
    variant_id      UUID NOT NULL REFERENCES variants(id) ON DELETE CASCADE,
    option_value_id UUID NOT NULL REFERENCES option_values(id),
    PRIMARY KEY (variant_id, option_value_id)
);

CREATE TABLE variant_translations (
    variant_id   UUID NOT NULL REFERENCES variants(id) ON DELETE CASCADE,
    locale_code  TEXT NOT NULL REFERENCES locales(code),
    title        TEXT NOT NULL,
    description  TEXT,
    PRIMARY KEY (variant_id, locale_code)
);

-- ---------------------------------------------------------------------------
-- Assets
-- ---------------------------------------------------------------------------

CREATE TABLE assets (
    id         UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID    NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    variant_id UUID    REFERENCES variants(id) ON DELETE CASCADE,  -- NULL = product-scoped
    url        TEXT    NOT NULL,
    type       TEXT    NOT NULL CHECK (type IN ('image','video')),
    sort_order INTEGER NOT NULL DEFAULT 0
);

-- ---------------------------------------------------------------------------
-- Channel & marketplace listings
-- ---------------------------------------------------------------------------

CREATE TABLE channel_products (
    channel_id     UUID           NOT NULL REFERENCES channels(id),
    product_id     UUID           NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    enabled        BOOLEAN        NOT NULL DEFAULT FALSE,
    price_override NUMERIC(12, 4),
    currency       TEXT,
    version        INTEGER        NOT NULL DEFAULT 1,
    PRIMARY KEY (channel_id, product_id)
);

CREATE TABLE marketplace_products (
    marketplace_id  UUID    NOT NULL REFERENCES marketplaces(id),
    product_id      UUID    NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    external_id     TEXT    NOT NULL,
    feed_attributes JSONB   NOT NULL DEFAULT '{}',
    version         INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (marketplace_id, product_id)
);

-- ---------------------------------------------------------------------------
-- Stock levels
-- ---------------------------------------------------------------------------

CREATE TABLE stock_levels (
    variant_id     UUID    NOT NULL REFERENCES variants(id) ON DELETE CASCADE,
    warehouse_code TEXT    NOT NULL,
    level          TEXT    NOT NULL DEFAULT 'OOS'
                           CHECK (level IN ('IN_STOCK','LOW','OOS')),
    version        INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (variant_id, warehouse_code)
);

-- ---------------------------------------------------------------------------
-- Transactional outbox
-- ---------------------------------------------------------------------------

CREATE TABLE outbox (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate     TEXT        NOT NULL,
    aggregate_id  UUID        NOT NULL,
    event_type    TEXT        NOT NULL,
    payload       JSONB       NOT NULL,
    trace_id      TEXT,
    span_id       TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    dispatched_at TIMESTAMPTZ
);

CREATE INDEX idx_outbox_undispatched ON outbox (created_at)
    WHERE dispatched_at IS NULL;

-- migrate:down

DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS stock_levels;
DROP TABLE IF EXISTS marketplace_products;
DROP TABLE IF EXISTS channel_products;
DROP TABLE IF EXISTS assets;
DROP TABLE IF EXISTS variant_translations;
DROP TABLE IF EXISTS variant_option_values;
DROP TABLE IF EXISTS variants;
DROP TABLE IF EXISTS option_value_translations;
DROP TABLE IF EXISTS option_values;
DROP TABLE IF EXISTS option_translations;
DROP TABLE IF EXISTS options;
DROP TABLE IF EXISTS product_translations;
DROP TABLE IF EXISTS products;
DROP TABLE IF EXISTS snapshots;
DROP TABLE IF EXISTS marketplaces;
DROP TABLE IF EXISTS channels;
DROP TABLE IF EXISTS locales;

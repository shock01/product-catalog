-- migrate:up

-- ---------------------------------------------------------------------------
-- Brands (reference table, synced from external service)
-- All products must belong to a brand.
-- No display name for now — code is the identifier.
-- ---------------------------------------------------------------------------

CREATE TABLE brands (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    code       TEXT        NOT NULL UNIQUE,
    active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- Taxonomy nodes (reference table, synced from external service)
-- Each node has an optional parent, forming a tree of up to 4 levels.
-- The numeric path (e.g. {1, 120, 780}) is stored on product_taxonomies,
-- not computed at query time.
-- ---------------------------------------------------------------------------

CREATE TABLE taxonomies (
    id         INTEGER     PRIMARY KEY,  -- numeric ID as used in paths
    parent_id  INTEGER     REFERENCES taxonomies(id),
    code       TEXT        NOT NULL UNIQUE,
    depth      SMALLINT    NOT NULL CHECK (depth BETWEEN 1 AND 4),
    active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE taxonomy_translations (
    taxonomy_id INTEGER NOT NULL REFERENCES taxonomies(id) ON DELETE CASCADE,
    locale_code TEXT    NOT NULL REFERENCES locales(code),
    name        TEXT    NOT NULL,
    PRIMARY KEY (taxonomy_id, locale_code)
);

-- ---------------------------------------------------------------------------
-- Product taxonomy assignments
-- A product may belong to multiple taxonomy nodes.
-- path stores the full ancestor chain as an integer array,
-- e.g. {1, 120, 780} where 780 is the leaf and 1 is the root.
-- Written by the importer/API at write time — not computed at query time.
-- ---------------------------------------------------------------------------

CREATE TABLE product_taxonomies (
    product_id  UUID      NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    taxonomy_id INTEGER   NOT NULL REFERENCES taxonomies(id),
    path        INTEGER[] NOT NULL,  -- full ancestor chain including leaf, e.g. {1, 120, 780}
    PRIMARY KEY (product_id, taxonomy_id)
);

-- GIN index for array containment queries (e.g. WHERE path @> '{1,120}')
CREATE INDEX idx_product_taxonomies_path ON product_taxonomies USING GIN (path);

-- ---------------------------------------------------------------------------
-- Add brand_id to products (NOT NULL — all products belong to a brand)
-- ---------------------------------------------------------------------------

ALTER TABLE products
    ADD COLUMN brand_id UUID NOT NULL REFERENCES brands(id);

-- migrate:down

ALTER TABLE products DROP COLUMN brand_id;
DROP INDEX IF EXISTS idx_product_taxonomies_path;
DROP TABLE IF EXISTS product_taxonomies;
DROP TABLE IF EXISTS taxonomy_translations;
DROP TABLE IF EXISTS taxonomies;
DROP TABLE IF EXISTS brands;

-- migrate:up

-- ---------------------------------------------------------------------------
-- Price snapshot imports
-- One row per S3 file processed (Athena dump or Kafka stream landing).
-- Same write-guard pattern as snapshots.
-- ---------------------------------------------------------------------------

CREATE TABLE price_snapshot_imports (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    s3_uri      TEXT        NOT NULL UNIQUE,
    dumped_at   TIMESTAMPTZ NOT NULL,
    imported_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    row_count   INTEGER,
    status      TEXT        NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending','processing','completed','failed'))
);

-- ---------------------------------------------------------------------------
-- Price snapshots (append-only)
-- One row per product × channel × marketplace price point.
-- channel_id NULL  = applies to all channels.
-- marketplace_id NULL = applies to all marketplaces.
-- Resolution priority (most specific wins):
--   channel + marketplace > channel only > marketplace only > global (both NULL)
-- ---------------------------------------------------------------------------

CREATE TABLE price_snapshots (
    id                    UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id            UUID           NOT NULL REFERENCES products(id),
    channel_id            UUID           REFERENCES channels(id),
    marketplace_id        UUID           REFERENCES marketplaces(id),
    amount                NUMERIC(12, 4) NOT NULL,
    currency              TEXT           NOT NULL,  -- ISO 4217
    price_snapshot_import_id UUID        NOT NULL REFERENCES price_snapshot_imports(id),
    effective_from        TIMESTAMPTZ    NOT NULL,
    created_at            TIMESTAMPTZ    NOT NULL DEFAULT now()
);

-- Lookup index: latest price for a given product × channel × marketplace
CREATE INDEX idx_price_snapshots_lookup
    ON price_snapshots (product_id, channel_id, marketplace_id, effective_from DESC);

-- ---------------------------------------------------------------------------
-- View: current effective price per product × channel × marketplace
-- Resolves the most specific price with the latest effective_from <= now().
-- Falls back to channel_products.price_override then variants.price.
-- ---------------------------------------------------------------------------

CREATE VIEW v_current_prices AS
WITH ranked AS (
    SELECT
        ps.product_id,
        ps.channel_id,
        ps.marketplace_id,
        ps.amount,
        ps.currency,
        ps.effective_from,
        -- specificity: channel+marketplace=3, channel-only=2, marketplace-only=1, global=0
        (CASE WHEN ps.channel_id IS NOT NULL THEN 2 ELSE 0 END +
         CASE WHEN ps.marketplace_id IS NOT NULL THEN 1 ELSE 0 END) AS specificity,
        ROW_NUMBER() OVER (
            PARTITION BY ps.product_id, ps.channel_id, ps.marketplace_id
            ORDER BY ps.effective_from DESC
        ) AS rn
    FROM price_snapshots ps
    WHERE ps.effective_from <= now()
),
best AS (
    SELECT *
    FROM ranked
    WHERE rn = 1
),
-- For each product+channel, pick the most specific price available
resolved AS (
    SELECT DISTINCT ON (p.id, cp.channel_id)
        p.id            AS product_id,
        cp.channel_id,
        NULL::uuid      AS marketplace_id,
        COALESCE(
            -- channel+marketplace match (marketplace_id NULL here since we resolve per-channel)
            (SELECT b.amount FROM best b
             WHERE b.product_id = p.id AND b.channel_id = cp.channel_id AND b.marketplace_id IS NULL
             ORDER BY b.specificity DESC LIMIT 1),
            -- global price
            (SELECT b.amount FROM best b
             WHERE b.product_id = p.id AND b.channel_id IS NULL AND b.marketplace_id IS NULL
             LIMIT 1),
            -- legacy fallback: channel price_override
            cp.price_override,
            -- legacy fallback: variant base price (min across variants)
            (SELECT MIN(v.price) FROM variants v WHERE v.product_id = p.id)
        )                AS amount,
        COALESCE(
            (SELECT b.currency FROM best b
             WHERE b.product_id = p.id AND b.channel_id = cp.channel_id AND b.marketplace_id IS NULL
             ORDER BY b.specificity DESC LIMIT 1),
            (SELECT b.currency FROM best b
             WHERE b.product_id = p.id AND b.channel_id IS NULL AND b.marketplace_id IS NULL
             LIMIT 1),
            cp.currency,
            (SELECT v.currency FROM variants v WHERE v.product_id = p.id LIMIT 1)
        )                AS currency
    FROM products p
    JOIN channel_products cp ON cp.product_id = p.id
)
SELECT * FROM resolved;

-- ---------------------------------------------------------------------------
-- View: current product snapshot
-- Joins products with their latest snapshot metadata.
-- Used by the API (GetByID) and the indexer instead of querying products directly.
-- ---------------------------------------------------------------------------

CREATE VIEW v_products_current AS
SELECT
    p.id,
    p.status,
    p.version,
    p.created_at,
    p.updated_at,
    p.deleted_at,
    s.id          AS snapshot_id,
    s.dumped_at   AS snapshot_dumped_at,
    s.s3_uri      AS snapshot_s3_uri
FROM products p
LEFT JOIN snapshots s ON s.id = p.snapshot_id;

-- migrate:down

DROP VIEW IF EXISTS v_products_current;
DROP VIEW IF EXISTS v_current_prices;
DROP INDEX IF EXISTS idx_price_snapshots_lookup;
DROP TABLE IF EXISTS price_snapshots;
DROP TABLE IF EXISTS price_snapshot_imports;

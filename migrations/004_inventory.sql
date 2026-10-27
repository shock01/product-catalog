-- migrate:up

-- Add updated_at to stock_levels for observability (when did this level last change).
-- Not required for correctness — the current level is always the row value.
ALTER TABLE stock_levels
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- migrate:down

ALTER TABLE stock_levels DROP COLUMN updated_at;

-- migrate:up
-- Seed: reference data + two sample products (T-Shirt and Hoodie)

-- ---------------------------------------------------------------------------
-- Locales
-- ---------------------------------------------------------------------------
INSERT INTO locales (code) VALUES
    ('en-GB'),
    ('de-DE'),
    ('nl-NL');

-- ---------------------------------------------------------------------------
-- Channels
-- ---------------------------------------------------------------------------
INSERT INTO channels (id, code, name, active) VALUES
    ('00000000-0000-0000-0000-000000000001', 'web', 'Web Store',       TRUE),
    ('00000000-0000-0000-0000-000000000002', 'pos', 'Point of Sale',   TRUE),
    ('00000000-0000-0000-0000-000000000003', 'app', 'Mobile App',      TRUE);

-- ---------------------------------------------------------------------------
-- Marketplaces
-- ---------------------------------------------------------------------------
INSERT INTO marketplaces (id, code, name, active) VALUES
    ('00000000-0000-0000-0000-000000000010', 'amazon-uk',  'Amazon UK',  TRUE),
    ('00000000-0000-0000-0000-000000000011', 'amazon-de',  'Amazon DE',  TRUE);

-- ---------------------------------------------------------------------------
-- Product 1: Classic T-Shirt (published, Size × Colour)
-- ---------------------------------------------------------------------------
INSERT INTO products (id, status, version) VALUES
    ('10000000-0000-0000-0000-000000000001', 'published', 1);

INSERT INTO product_translations (product_id, locale_code, title, slug, description, short_description, meta_title, meta_description) VALUES
    ('10000000-0000-0000-0000-000000000001', 'en-GB', 'Classic T-Shirt',       'classic-t-shirt',       'A timeless wardrobe staple crafted from 100% organic cotton.', 'Everyday comfort.',          'Classic T-Shirt | MyStore',       'Shop the Classic T-Shirt in multiple colours and sizes.'),
    ('10000000-0000-0000-0000-000000000001', 'de-DE', 'Klassisches T-Shirt',   'klassisches-t-shirt',   'Ein zeitloser Klassiker aus 100% Bio-Baumwolle.',              'Alltäglicher Komfort.',      'Klassisches T-Shirt | MyStore',   'Kaufen Sie das klassische T-Shirt in verschiedenen Farben und Größen.'),
    ('10000000-0000-0000-0000-000000000001', 'nl-NL', 'Klassiek T-Shirt',      'klassiek-t-shirt',      'Een tijdloos kledingstuk van 100% biologisch katoen.',         'Dagelijks comfort.',         'Klassiek T-Shirt | MyStore',      'Shop het Klassieke T-Shirt in meerdere kleuren en maten.');

-- Options: Size
INSERT INTO options (id, product_id, code, sort_order) VALUES
    ('20000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', 'size',   0);

INSERT INTO option_translations (option_id, locale_code, name) VALUES
    ('20000000-0000-0000-0000-000000000001', 'en-GB', 'Size'),
    ('20000000-0000-0000-0000-000000000001', 'de-DE', 'Größe'),
    ('20000000-0000-0000-0000-000000000001', 'nl-NL', 'Maat');

INSERT INTO option_values (id, option_id, code, sort_order) VALUES
    ('30000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001', 's',  0),
    ('30000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000001', 'm',  1),
    ('30000000-0000-0000-0000-000000000003', '20000000-0000-0000-0000-000000000001', 'l',  2),
    ('30000000-0000-0000-0000-000000000004', '20000000-0000-0000-0000-000000000001', 'xl', 3);

INSERT INTO option_value_translations (option_value_id, locale_code, value) VALUES
    ('30000000-0000-0000-0000-000000000001', 'en-GB', 'S'),  ('30000000-0000-0000-0000-000000000001', 'de-DE', 'S'),  ('30000000-0000-0000-0000-000000000001', 'nl-NL', 'S'),
    ('30000000-0000-0000-0000-000000000002', 'en-GB', 'M'),  ('30000000-0000-0000-0000-000000000002', 'de-DE', 'M'),  ('30000000-0000-0000-0000-000000000002', 'nl-NL', 'M'),
    ('30000000-0000-0000-0000-000000000003', 'en-GB', 'L'),  ('30000000-0000-0000-0000-000000000003', 'de-DE', 'L'),  ('30000000-0000-0000-0000-000000000003', 'nl-NL', 'L'),
    ('30000000-0000-0000-0000-000000000004', 'en-GB', 'XL'), ('30000000-0000-0000-0000-000000000004', 'de-DE', 'XL'), ('30000000-0000-0000-0000-000000000004', 'nl-NL', 'XL');

-- Options: Colour
INSERT INTO options (id, product_id, code, sort_order) VALUES
    ('20000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001', 'colour', 1);

INSERT INTO option_translations (option_id, locale_code, name) VALUES
    ('20000000-0000-0000-0000-000000000002', 'en-GB', 'Colour'),
    ('20000000-0000-0000-0000-000000000002', 'de-DE', 'Farbe'),
    ('20000000-0000-0000-0000-000000000002', 'nl-NL', 'Kleur');

INSERT INTO option_values (id, option_id, code, sort_order) VALUES
    ('30000000-0000-0000-0000-000000000010', '20000000-0000-0000-0000-000000000002', 'red',  0),
    ('30000000-0000-0000-0000-000000000011', '20000000-0000-0000-0000-000000000002', 'blue', 1);

INSERT INTO option_value_translations (option_value_id, locale_code, value) VALUES
    ('30000000-0000-0000-0000-000000000010', 'en-GB', 'Red'),  ('30000000-0000-0000-0000-000000000010', 'de-DE', 'Rot'),  ('30000000-0000-0000-0000-000000000010', 'nl-NL', 'Rood'),
    ('30000000-0000-0000-0000-000000000011', 'en-GB', 'Blue'), ('30000000-0000-0000-0000-000000000011', 'de-DE', 'Blau'), ('30000000-0000-0000-0000-000000000011', 'nl-NL', 'Blauw');

-- Variants: S/Red, S/Blue, M/Red, M/Blue, L/Red, L/Blue, XL/Blue only
INSERT INTO variants (id, product_id, sku, price, currency) VALUES
    ('40000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', 'TSHIRT-S-RED',  19.99, 'GBP'),
    ('40000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001', 'TSHIRT-S-BLU',  19.99, 'GBP'),
    ('40000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000001', 'TSHIRT-M-RED',  19.99, 'GBP'),
    ('40000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000001', 'TSHIRT-M-BLU',  19.99, 'GBP'),
    ('40000000-0000-0000-0000-000000000005', '10000000-0000-0000-0000-000000000001', 'TSHIRT-L-RED',  19.99, 'GBP'),
    ('40000000-0000-0000-0000-000000000006', '10000000-0000-0000-0000-000000000001', 'TSHIRT-L-BLU',  19.99, 'GBP'),
    ('40000000-0000-0000-0000-000000000007', '10000000-0000-0000-0000-000000000001', 'TSHIRT-XL-BLU', 21.99, 'GBP');

INSERT INTO variant_option_values (variant_id, option_value_id) VALUES
    ('40000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001'), -- S
    ('40000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000010'), -- Red
    ('40000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000001'), -- S
    ('40000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000011'), -- Blue
    ('40000000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000002'), -- M
    ('40000000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000010'), -- Red
    ('40000000-0000-0000-0000-000000000004', '30000000-0000-0000-0000-000000000002'), -- M
    ('40000000-0000-0000-0000-000000000004', '30000000-0000-0000-0000-000000000011'), -- Blue
    ('40000000-0000-0000-0000-000000000005', '30000000-0000-0000-0000-000000000003'), -- L
    ('40000000-0000-0000-0000-000000000005', '30000000-0000-0000-0000-000000000010'), -- Red
    ('40000000-0000-0000-0000-000000000006', '30000000-0000-0000-0000-000000000003'), -- L
    ('40000000-0000-0000-0000-000000000006', '30000000-0000-0000-0000-000000000011'), -- Blue
    ('40000000-0000-0000-0000-000000000007', '30000000-0000-0000-0000-000000000004'), -- XL
    ('40000000-0000-0000-0000-000000000007', '30000000-0000-0000-0000-000000000011'); -- Blue

INSERT INTO variant_translations (variant_id, locale_code, title) VALUES
    ('40000000-0000-0000-0000-000000000001', 'en-GB', 'Classic T-Shirt — S / Red'),
    ('40000000-0000-0000-0000-000000000002', 'en-GB', 'Classic T-Shirt — S / Blue'),
    ('40000000-0000-0000-0000-000000000003', 'en-GB', 'Classic T-Shirt — M / Red'),
    ('40000000-0000-0000-0000-000000000004', 'en-GB', 'Classic T-Shirt — M / Blue'),
    ('40000000-0000-0000-0000-000000000005', 'en-GB', 'Classic T-Shirt — L / Red'),
    ('40000000-0000-0000-0000-000000000006', 'en-GB', 'Classic T-Shirt — L / Blue'),
    ('40000000-0000-0000-0000-000000000007', 'en-GB', 'Classic T-Shirt — XL / Blue');

INSERT INTO stock_levels (variant_id, warehouse_code, level) VALUES
    ('40000000-0000-0000-0000-000000000001', 'WH-LON-01', 'IN_STOCK'),
    ('40000000-0000-0000-0000-000000000002', 'WH-LON-01', 'IN_STOCK'),
    ('40000000-0000-0000-0000-000000000003', 'WH-LON-01', 'IN_STOCK'),
    ('40000000-0000-0000-0000-000000000004', 'WH-LON-01', 'LOW'),
    ('40000000-0000-0000-0000-000000000005', 'WH-LON-01', 'IN_STOCK'),
    ('40000000-0000-0000-0000-000000000006', 'WH-LON-01', 'OOS'),
    ('40000000-0000-0000-0000-000000000007', 'WH-LON-01', 'IN_STOCK');

INSERT INTO channel_products (channel_id, product_id, enabled, price_override, currency) VALUES
    ('00000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', TRUE,  NULL,  NULL),
    ('00000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001', TRUE,  17.99, 'GBP'),
    ('00000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000001', TRUE,  NULL,  NULL);

INSERT INTO assets (id, product_id, variant_id, url, type, sort_order) VALUES
    ('50000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', NULL,                                       'https://cdn.example.com/tshirt-hero.jpg',  'image', 0),
    ('50000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000001', '40000000-0000-0000-0000-000000000001',     'https://cdn.example.com/tshirt-s-red.jpg', 'image', 0),
    ('50000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000001', '40000000-0000-0000-0000-000000000002',     'https://cdn.example.com/tshirt-s-blu.jpg', 'image', 0);

-- ---------------------------------------------------------------------------
-- Product 2: Classic Hoodie (active/draft — not yet published, no search index)
-- ---------------------------------------------------------------------------
INSERT INTO products (id, status, version) VALUES
    ('10000000-0000-0000-0000-000000000002', 'active', 1);

INSERT INTO product_translations (product_id, locale_code, title, slug, description, short_description, meta_title, meta_description) VALUES
    ('10000000-0000-0000-0000-000000000002', 'en-GB', 'Classic Hoodie', 'classic-hoodie', 'A cosy pullover hoodie for all seasons.', 'Warm and comfortable.', 'Classic Hoodie | MyStore', 'Shop the Classic Hoodie.');

INSERT INTO options (id, product_id, code, sort_order) VALUES
    ('20000000-0000-0000-0000-000000000010', '10000000-0000-0000-0000-000000000002', 'size', 0);

INSERT INTO option_translations (option_id, locale_code, name) VALUES
    ('20000000-0000-0000-0000-000000000010', 'en-GB', 'Size');

INSERT INTO option_values (id, option_id, code, sort_order) VALUES
    ('30000000-0000-0000-0000-000000000020', '20000000-0000-0000-0000-000000000010', 'm',  0),
    ('30000000-0000-0000-0000-000000000021', '20000000-0000-0000-0000-000000000010', 'l',  1),
    ('30000000-0000-0000-0000-000000000022', '20000000-0000-0000-0000-000000000010', 'xl', 2);

INSERT INTO option_value_translations (option_value_id, locale_code, value) VALUES
    ('30000000-0000-0000-0000-000000000020', 'en-GB', 'M'),
    ('30000000-0000-0000-0000-000000000021', 'en-GB', 'L'),
    ('30000000-0000-0000-0000-000000000022', 'en-GB', 'XL');

INSERT INTO variants (id, product_id, sku, price, currency) VALUES
    ('40000000-0000-0000-0000-000000000010', '10000000-0000-0000-0000-000000000002', 'HOODIE-M',  39.99, 'GBP'),
    ('40000000-0000-0000-0000-000000000011', '10000000-0000-0000-0000-000000000002', 'HOODIE-L',  39.99, 'GBP'),
    ('40000000-0000-0000-0000-000000000012', '10000000-0000-0000-0000-000000000002', 'HOODIE-XL', 39.99, 'GBP');

INSERT INTO variant_option_values (variant_id, option_value_id) VALUES
    ('40000000-0000-0000-0000-000000000010', '30000000-0000-0000-0000-000000000020'),
    ('40000000-0000-0000-0000-000000000011', '30000000-0000-0000-0000-000000000021'),
    ('40000000-0000-0000-0000-000000000012', '30000000-0000-0000-0000-000000000022');

INSERT INTO variant_translations (variant_id, locale_code, title) VALUES
    ('40000000-0000-0000-0000-000000000010', 'en-GB', 'Classic Hoodie — M'),
    ('40000000-0000-0000-0000-000000000011', 'en-GB', 'Classic Hoodie — L'),
    ('40000000-0000-0000-0000-000000000012', 'en-GB', 'Classic Hoodie — XL');

INSERT INTO stock_levels (variant_id, warehouse_code, level) VALUES
    ('40000000-0000-0000-0000-000000000010', 'WH-LON-01', 'IN_STOCK'),
    ('40000000-0000-0000-0000-000000000011', 'WH-LON-01', 'IN_STOCK'),
    ('40000000-0000-0000-0000-000000000012', 'WH-LON-01', 'LOW');

INSERT INTO channel_products (channel_id, product_id, enabled) VALUES
    ('00000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000002', FALSE);

-- migrate:down

DELETE FROM stock_levels       WHERE variant_id  LIKE '4%';
DELETE FROM variant_option_values WHERE variant_id LIKE '4%';
DELETE FROM variant_translations  WHERE variant_id LIKE '4%';
DELETE FROM variants           WHERE product_id  LIKE '1%';
DELETE FROM option_value_translations WHERE option_value_id LIKE '3%';
DELETE FROM option_value_translations WHERE option_value_id LIKE '3%';
DELETE FROM option_values      WHERE option_id   LIKE '2%';
DELETE FROM option_translations WHERE option_id  LIKE '2%';
DELETE FROM options            WHERE product_id  LIKE '1%';
DELETE FROM product_translations WHERE product_id LIKE '1%';
DELETE FROM channel_products   WHERE product_id  LIKE '1%';
DELETE FROM assets             WHERE product_id  LIKE '1%';
DELETE FROM products           WHERE id          LIKE '1%';
DELETE FROM marketplaces       WHERE id          LIKE '0%';
DELETE FROM channels           WHERE id          LIKE '0%';
DELETE FROM locales;

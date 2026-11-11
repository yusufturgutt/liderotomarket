PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS products (
    id               TEXT PRIMARY KEY,
    title            TEXT NOT NULL,
    slug             TEXT NOT NULL,
    description_html TEXT NOT NULL DEFAULT '',
    description_text TEXT NOT NULL DEFAULT '',
    shopier_url      TEXT NOT NULL DEFAULT '',
    price            INTEGER NOT NULL,       -- kuruş
    old_price        INTEGER NOT NULL DEFAULT 0,
    currency         TEXT NOT NULL DEFAULT 'TRY',
    shipping_price   INTEGER NOT NULL DEFAULT 0,
    in_stock         INTEGER NOT NULL DEFAULT 1,
    stock_quantity   INTEGER NOT NULL DEFAULT 0,
    placement_score  INTEGER NOT NULL DEFAULT 0,
    created_at       TEXT NOT NULL DEFAULT '',
    updated_at       TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS categories (
    id    TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    slug  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS product_categories (
    product_id  TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL,
    PRIMARY KEY (product_id, category_id)
);

CREATE TABLE IF NOT EXISTS product_images (
    product_id       TEXT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    placement        INTEGER NOT NULL,
    source_url       TEXT NOT NULL,
    thumb_source_url TEXT NOT NULL DEFAULT '',
    path             TEXT NOT NULL,
    thumb_path       TEXT NOT NULL,
    PRIMARY KEY (product_id, placement)
);

CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

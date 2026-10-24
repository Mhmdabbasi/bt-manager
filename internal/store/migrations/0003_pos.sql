CREATE TABLE IF NOT EXISTS voucher_products (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  profile_name TEXT NOT NULL,
  price_minor INTEGER NOT NULL CHECK(price_minor > 0),
  active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1)),
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS pos_sales (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  reference TEXT NOT NULL UNIQUE,
  total_minor INTEGER NOT NULL CHECK(total_minor > 0),
  currency_code TEXT NOT NULL CHECK(length(currency_code)=3),
  payment_method TEXT NOT NULL CHECK(payment_method IN ('cash','transfer','card','other')),
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS pos_sale_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  sale_id INTEGER NOT NULL REFERENCES pos_sales(id) ON DELETE RESTRICT,
  voucher_id INTEGER NOT NULL UNIQUE REFERENCES vouchers(id) ON DELETE RESTRICT,
  product_id INTEGER NOT NULL REFERENCES voucher_products(id) ON DELETE RESTRICT,
  product_name TEXT NOT NULL,
  profile_name TEXT NOT NULL,
  unit_price_minor INTEGER NOT NULL CHECK(unit_price_minor > 0),
  currency_code TEXT NOT NULL CHECK(length(currency_code)=3)
);

ALTER TABLE vouchers ADD COLUMN sale_id INTEGER REFERENCES pos_sales(id);
ALTER TABLE vouchers ADD COLUMN sold_at TEXT;

CREATE INDEX IF NOT EXISTS idx_pos_sales_created ON pos_sales(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_pos_sale_items_sale_id ON pos_sale_items(sale_id);
CREATE INDEX IF NOT EXISTS idx_voucher_products_active ON voucher_products(active,profile_name);

CREATE UNIQUE INDEX IF NOT EXISTS idx_voucher_products_profile_unique ON voucher_products(profile_name);

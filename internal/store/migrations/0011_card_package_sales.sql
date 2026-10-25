-- Package-backed point-of-sale sales are kept separate from the legacy voucher_products flow.
-- Each sold voucher and stock movement is recorded in the same SQLite transaction.
CREATE TABLE IF NOT EXISTS card_package_sales (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  reference TEXT NOT NULL UNIQUE,
  point_of_sale_id INTEGER NOT NULL REFERENCES points_of_sale(id) ON DELETE RESTRICT,
  package_id INTEGER NOT NULL REFERENCES card_packages(id) ON DELETE RESTRICT,
  total_minor INTEGER NOT NULL CHECK(total_minor > 0),
  currency_code TEXT NOT NULL CHECK(length(currency_code)=3),
  payment_method TEXT NOT NULL CHECK(payment_method IN ('cash','transfer','card','other')),
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE TABLE IF NOT EXISTS card_package_sale_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  sale_id INTEGER NOT NULL REFERENCES card_package_sales(id) ON DELETE RESTRICT,
  voucher_id INTEGER NOT NULL UNIQUE REFERENCES vouchers(id) ON DELETE RESTRICT,
  batch_id INTEGER NOT NULL REFERENCES voucher_batches(id) ON DELETE RESTRICT,
  package_id INTEGER NOT NULL REFERENCES card_packages(id) ON DELETE RESTRICT,
  username TEXT NOT NULL,
  package_name TEXT NOT NULL,
  unit_price_minor INTEGER NOT NULL CHECK(unit_price_minor > 0),
  currency_code TEXT NOT NULL CHECK(length(currency_code)=3)
);
CREATE INDEX IF NOT EXISTS idx_card_package_sales_pos_created
  ON card_package_sales(point_of_sale_id,created_at DESC);
CREATE INDEX IF NOT EXISTS idx_card_package_sale_items_sale
  ON card_package_sale_items(sale_id,id);

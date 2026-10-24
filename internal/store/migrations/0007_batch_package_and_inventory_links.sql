-- Phase 0 foundation: connect new domain records to batches and voucher inventory.
-- Existing records keep NULL links and remain readable; no legacy product/sale rows are deleted.

ALTER TABLE voucher_batches ADD COLUMN router_id INTEGER REFERENCES routers(id) ON DELETE RESTRICT;
ALTER TABLE voucher_batches ADD COLUMN package_id INTEGER REFERENCES card_packages(id) ON DELETE RESTRICT;
ALTER TABLE voucher_batches ADD COLUMN point_of_sale_id INTEGER REFERENCES points_of_sale(id) ON DELETE RESTRICT;
ALTER TABLE voucher_batches ADD COLUMN template_version_id INTEGER REFERENCES card_template_versions(id) ON DELETE RESTRICT;
ALTER TABLE voucher_batches ADD COLUMN service_system_snapshot TEXT NOT NULL DEFAULT 'hotspot' CHECK(service_system_snapshot IN ('hotspot','usermanager'));
ALTER TABLE voucher_batches ADD COLUMN package_name_snapshot TEXT NOT NULL DEFAULT '';
ALTER TABLE voucher_batches ADD COLUMN user_price_minor_snapshot INTEGER CHECK(user_price_minor_snapshot IS NULL OR user_price_minor_snapshot >= 0);
ALTER TABLE voucher_batches ADD COLUMN dealer_price_minor_snapshot INTEGER CHECK(dealer_price_minor_snapshot IS NULL OR dealer_price_minor_snapshot >= 0);
ALTER TABLE voucher_batches ADD COLUMN generation_config_json TEXT NOT NULL DEFAULT '{}';

ALTER TABLE vouchers ADD COLUMN package_id INTEGER REFERENCES card_packages(id) ON DELETE RESTRICT;
ALTER TABLE vouchers ADD COLUMN point_of_sale_id INTEGER REFERENCES points_of_sale(id) ON DELETE RESTRICT;

CREATE INDEX IF NOT EXISTS idx_voucher_batches_package ON voucher_batches(package_id,id);
CREATE INDEX IF NOT EXISTS idx_voucher_batches_pos ON voucher_batches(point_of_sale_id,id);
CREATE INDEX IF NOT EXISTS idx_voucher_batches_template_version ON voucher_batches(template_version_id);
CREATE INDEX IF NOT EXISTS idx_vouchers_package_pos ON vouchers(package_id,point_of_sale_id,status);

-- Inventory events are separate from print jobs and financial settlement. A successful print is
-- not a sale, and assigning stock to a dealer is not evidence that they have paid for it.
CREATE TABLE IF NOT EXISTS voucher_stock_movements (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  reference TEXT NOT NULL UNIQUE,
  movement_type TEXT NOT NULL CHECK(movement_type IN ('issue','allocate','transfer','return','sale','void','adjustment')),
  batch_id INTEGER NOT NULL REFERENCES voucher_batches(id) ON DELETE RESTRICT,
  package_id INTEGER REFERENCES card_packages(id) ON DELETE RESTRICT,
  from_point_of_sale_id INTEGER REFERENCES points_of_sale(id) ON DELETE RESTRICT,
  to_point_of_sale_id INTEGER REFERENCES points_of_sale(id) ON DELETE RESTRICT,
  quantity INTEGER NOT NULL CHECK(quantity > 0),
  note TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  CHECK(from_point_of_sale_id IS NULL OR to_point_of_sale_id IS NULL OR from_point_of_sale_id <> to_point_of_sale_id)
);
CREATE INDEX IF NOT EXISTS idx_voucher_stock_movements_batch
  ON voucher_stock_movements(batch_id,id);
CREATE INDEX IF NOT EXISTS idx_voucher_stock_movements_from_pos
  ON voucher_stock_movements(from_point_of_sale_id,id);
CREATE INDEX IF NOT EXISTS idx_voucher_stock_movements_to_pos
  ON voucher_stock_movements(to_point_of_sale_id,id);

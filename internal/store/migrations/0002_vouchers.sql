CREATE TABLE IF NOT EXISTS voucher_batches (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  reference TEXT NOT NULL UNIQUE,
  prefix TEXT NOT NULL DEFAULT '',
  profile_name TEXT NOT NULL,
  server_name TEXT NOT NULL DEFAULT 'all',
  limit_uptime TEXT NOT NULL DEFAULT '',
  requested_count INTEGER NOT NULL CHECK(requested_count BETWEEN 1 AND 200),
  created_count INTEGER NOT NULL DEFAULT 0 CHECK(created_count >= 0),
  failed_count INTEGER NOT NULL DEFAULT 0 CHECK(failed_count >= 0),
  status TEXT NOT NULL CHECK(status IN ('running','completed','partial','failed')),
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE IF NOT EXISTS vouchers (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  batch_id INTEGER NOT NULL REFERENCES voucher_batches(id) ON DELETE CASCADE,
  username TEXT NOT NULL UNIQUE,
  profile_name TEXT NOT NULL,
  server_name TEXT NOT NULL DEFAULT 'all',
  limit_uptime TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'available' CHECK(status IN ('available','sold','void')),
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE INDEX IF NOT EXISTS idx_vouchers_batch_id ON vouchers(batch_id);
CREATE INDEX IF NOT EXISTS idx_vouchers_status ON vouchers(status);
CREATE INDEX IF NOT EXISTS idx_voucher_batches_created ON voucher_batches(created_at DESC);

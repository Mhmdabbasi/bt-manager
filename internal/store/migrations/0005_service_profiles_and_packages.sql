-- Phase 0 foundation: imported service profiles and commercial card packages.
-- Additive only. Legacy voucher_products and already issued vouchers are preserved.
-- Router credentials are intentionally not stored here.

CREATE TABLE IF NOT EXISTS routers (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  router_key TEXT NOT NULL UNIQUE,
  display_name TEXT NOT NULL DEFAULT '',
  address TEXT NOT NULL,
  api_port INTEGER NOT NULL DEFAULT 8728 CHECK(api_port BETWEEN 1 AND 65535),
  use_tls INTEGER NOT NULL DEFAULT 0 CHECK(use_tls IN (0,1)),
  router_identity TEXT NOT NULL DEFAULT '',
  routeros_version TEXT NOT NULL DEFAULT '',
  model TEXT NOT NULL DEFAULT '',
  last_seen_at TEXT,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_routers_address_port ON routers(address,api_port);

-- Unified identity for imported HotSpot/User Manager profiles; raw properties preserve
-- version-specific fields, and missing profiles are soft-marked rather than deleted.
CREATE TABLE IF NOT EXISTS service_profiles (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  router_id INTEGER NOT NULL REFERENCES routers(id) ON DELETE RESTRICT,
  source_system TEXT NOT NULL CHECK(source_system IN ('hotspot','usermanager')),
  source_id TEXT NOT NULL,
  name TEXT NOT NULL,
  properties_json TEXT NOT NULL DEFAULT '{}',
  is_present INTEGER NOT NULL DEFAULT 1 CHECK(is_present IN (0,1)),
  last_seen_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  first_seen_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  UNIQUE(router_id,source_system,source_id)
);
CREATE INDEX IF NOT EXISTS idx_service_profiles_lookup
  ON service_profiles(router_id,source_system,is_present,name COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS um_limitations (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  router_id INTEGER NOT NULL REFERENCES routers(id) ON DELETE RESTRICT,
  source_id TEXT NOT NULL,
  name TEXT NOT NULL,
  properties_json TEXT NOT NULL DEFAULT '{}',
  is_present INTEGER NOT NULL DEFAULT 1 CHECK(is_present IN (0,1)),
  last_seen_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  first_seen_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  UNIQUE(router_id,source_id)
);
CREATE INDEX IF NOT EXISTS idx_um_limitations_lookup
  ON um_limitations(router_id,is_present,name COLLATE NOCASE);

-- A User Manager profile can link to multiple limitations; application code checks source_system.
CREATE TABLE IF NOT EXISTS um_profile_limitations (
  profile_id INTEGER NOT NULL REFERENCES service_profiles(id) ON DELETE RESTRICT,
  limitation_id INTEGER NOT NULL REFERENCES um_limitations(id) ON DELETE RESTRICT,
  position INTEGER NOT NULL DEFAULT 0 CHECK(position >= 0),
  properties_json TEXT NOT NULL DEFAULT '{}',
  PRIMARY KEY(profile_id,limitation_id)
);
CREATE INDEX IF NOT EXISTS idx_um_profile_limitations_limitation
  ON um_profile_limitations(limitation_id,profile_id);

CREATE TABLE IF NOT EXISTS card_packages (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  service_system TEXT NOT NULL CHECK(service_system IN ('hotspot','usermanager')),
  service_profile_id INTEGER NOT NULL REFERENCES service_profiles(id) ON DELETE RESTRICT,
  description TEXT NOT NULL DEFAULT '',
  user_price_minor INTEGER NOT NULL CHECK(user_price_minor > 0),
  dealer_price_minor INTEGER NOT NULL DEFAULT 0 CHECK(dealer_price_minor >= 0 AND dealer_price_minor <= user_price_minor),
  default_prefix TEXT NOT NULL DEFAULT '',
  generation_config_json TEXT NOT NULL DEFAULT '{}',
  active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1)),
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_card_packages_active_name
  ON card_packages(active,sort_order,name COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_card_packages_profile
  ON card_packages(service_profile_id);

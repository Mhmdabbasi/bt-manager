-- Phase 0 foundation: authorized points of sale and versioned printable card templates.
-- Asset bytes live under the application's managed data directory; only metadata is stored here.

CREATE TABLE IF NOT EXISTS points_of_sale (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  phone TEXT NOT NULL DEFAULT '',
  address TEXT NOT NULL DEFAULT '',
  notes TEXT NOT NULL DEFAULT '',
  package_selection_mode TEXT NOT NULL DEFAULT 'all' CHECK(package_selection_mode IN ('all','selected')),
  active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1)),
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_points_of_sale_active_name
  ON points_of_sale(active,name COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS design_assets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  asset_key TEXT NOT NULL UNIQUE,
  asset_kind TEXT NOT NULL DEFAULT 'background' CHECK(asset_kind IN ('background','image')),
  original_name TEXT NOT NULL,
  mime_type TEXT NOT NULL CHECK(mime_type IN ('image/png','image/jpeg','image/webp')),
  size_bytes INTEGER NOT NULL CHECK(size_bytes > 0),
  sha256 TEXT NOT NULL,
  width_px INTEGER CHECK(width_px IS NULL OR width_px > 0),
  height_px INTEGER CHECK(height_px IS NULL OR height_px > 0),
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- A template belongs to one card package and is either global or scoped to a single point of sale.
-- Multiple templates per package/scope are supported. Only one active global default may exist.
CREATE TABLE IF NOT EXISTS card_templates (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  package_id INTEGER NOT NULL REFERENCES card_packages(id) ON DELETE RESTRICT,
  point_of_sale_id INTEGER REFERENCES points_of_sale(id) ON DELETE RESTRICT,
  scope_kind TEXT NOT NULL DEFAULT 'global' CHECK(scope_kind IN ('global','point_of_sale')),
  name TEXT NOT NULL,
  is_default INTEGER NOT NULL DEFAULT 0 CHECK(is_default IN (0,1)),
  active INTEGER NOT NULL DEFAULT 1 CHECK(active IN (0,1)),
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  CHECK((scope_kind='global' AND point_of_sale_id IS NULL) OR
        (scope_kind='point_of_sale' AND point_of_sale_id IS NOT NULL AND is_default=0))
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_card_templates_global_default
  ON card_templates(package_id) WHERE scope_kind='global' AND is_default=1;
CREATE INDEX IF NOT EXISTS idx_card_templates_package_scope
  ON card_templates(package_id,scope_kind,point_of_sale_id,active);

-- Immutable versions preserve prior appearance for batches that must be reprinted later.
CREATE TABLE IF NOT EXISTS card_template_versions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  template_id INTEGER NOT NULL REFERENCES card_templates(id) ON DELETE RESTRICT,
  version_number INTEGER NOT NULL CHECK(version_number >= 1),
  card_width_mm REAL NOT NULL CHECK(card_width_mm > 0 AND card_width_mm <= 300),
  card_height_mm REAL NOT NULL CHECK(card_height_mm > 0 AND card_height_mm <= 300),
  design_json TEXT NOT NULL DEFAULT '{}',
  page_layout_json TEXT NOT NULL DEFAULT '{}',
  background_asset_id INTEGER REFERENCES design_assets(id) ON DELETE RESTRICT,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  UNIQUE(template_id,version_number)
);
CREATE INDEX IF NOT EXISTS idx_card_template_versions_template
  ON card_template_versions(template_id,version_number DESC);

-- package_selection_mode='all': absent rule means allowed; 'selected': absent rule means denied.
-- A null prefix/template override falls back to the package prefix/global default template.
CREATE TABLE IF NOT EXISTS point_of_sale_packages (
  point_of_sale_id INTEGER NOT NULL REFERENCES points_of_sale(id) ON DELETE RESTRICT,
  package_id INTEGER NOT NULL REFERENCES card_packages(id) ON DELETE RESTRICT,
  is_enabled INTEGER NOT NULL DEFAULT 1 CHECK(is_enabled IN (0,1)),
  prefix_override TEXT,
  template_id_override INTEGER REFERENCES card_templates(id) ON DELETE RESTRICT,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  PRIMARY KEY(point_of_sale_id,package_id)
);
CREATE INDEX IF NOT EXISTS idx_pos_packages_package
  ON point_of_sale_packages(package_id,point_of_sale_id);

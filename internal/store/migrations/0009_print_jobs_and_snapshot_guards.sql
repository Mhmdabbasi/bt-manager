-- Phase 0 finalization: print-job history, fuller batch snapshots, and immutable references.
-- Use a new migration rather than rewriting an already published schema step.

ALTER TABLE voucher_batches ADD COLUMN service_profile_snapshot_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE voucher_batches ADD COLUMN limitation_snapshot_json TEXT NOT NULL DEFAULT '[]';

-- Allow a replacement global default after the previous default has been archived.
DROP INDEX IF EXISTS idx_card_templates_global_default;
CREATE UNIQUE INDEX IF NOT EXISTS idx_card_templates_global_default
  ON card_templates(package_id)
  WHERE scope_kind='global' AND is_default=1 AND active=1;

CREATE TABLE IF NOT EXISTS print_jobs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  reference TEXT NOT NULL UNIQUE,
  batch_id INTEGER NOT NULL REFERENCES voucher_batches(id) ON DELETE RESTRICT,
  template_version_id INTEGER NOT NULL REFERENCES card_template_versions(id) ON DELETE RESTRICT,
  job_kind TEXT NOT NULL DEFAULT 'initial' CHECK(job_kind IN ('initial','reprint')),
  status TEXT NOT NULL CHECK(status IN ('queued','running','completed','failed')),
  output_key TEXT,
  page_count INTEGER CHECK(page_count IS NULL OR page_count >= 0),
  card_count INTEGER NOT NULL CHECK(card_count >= 0),
  error_message TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  completed_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_print_jobs_batch
  ON print_jobs(batch_id,id DESC);
CREATE INDEX IF NOT EXISTS idx_print_jobs_status
  ON print_jobs(status,created_at);

-- Do not mutate source identity once a package or relationship points to that profile.
-- Syncs update properties/presence, not the stable local/source identity tuple.
CREATE TRIGGER IF NOT EXISTS trg_service_profile_identity_guard
BEFORE UPDATE OF router_id,source_system,source_id ON service_profiles
FOR EACH ROW
WHEN (NEW.router_id<>OLD.router_id OR NEW.source_system<>OLD.source_system OR NEW.source_id<>OLD.source_id)
 AND (
   EXISTS(SELECT 1 FROM card_packages p WHERE p.service_profile_id=OLD.id)
   OR EXISTS(SELECT 1 FROM um_profile_limitations r WHERE r.profile_id=OLD.id)
 )
BEGIN
  SELECT RAISE(ABORT,'a referenced service profile identity cannot be changed');
END;

-- Once a template has a saved version, its package/scope is stable. Edit its design by creating
-- another version; archive it through active=0 instead of moving it between packages/POS records.
CREATE TRIGGER IF NOT EXISTS trg_card_template_scope_guard
BEFORE UPDATE OF package_id,point_of_sale_id,scope_kind ON card_templates
FOR EACH ROW
WHEN (NEW.package_id<>OLD.package_id
      OR NEW.point_of_sale_id IS NOT OLD.point_of_sale_id
      OR NEW.scope_kind<>OLD.scope_kind)
 AND EXISTS(SELECT 1 FROM card_template_versions v WHERE v.template_id=OLD.id)
BEGIN
  SELECT RAISE(ABORT,'a versioned template cannot be moved between package or point-of-sale scopes');
END;

-- An asset referenced by a template version must remain immutable and available for reprints.
CREATE TRIGGER IF NOT EXISTS trg_design_asset_immutable_update
BEFORE UPDATE ON design_assets
FOR EACH ROW
WHEN EXISTS(SELECT 1 FROM card_template_versions v WHERE v.background_asset_id=OLD.id)
BEGIN
  SELECT RAISE(ABORT,'an asset used by a saved template version cannot be modified');
END;

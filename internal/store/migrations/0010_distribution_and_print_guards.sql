-- Final cross-table guardrails for the new distribution and print-job domain.

CREATE TRIGGER IF NOT EXISTS trg_um_limitation_identity_guard
BEFORE UPDATE OF router_id,source_id ON um_limitations
FOR EACH ROW
WHEN (NEW.router_id<>OLD.router_id OR NEW.source_id<>OLD.source_id)
 AND EXISTS(SELECT 1 FROM um_profile_limitations r WHERE r.limitation_id=OLD.id)
BEGIN
  SELECT RAISE(ABORT,'a referenced limitation identity cannot be changed');
END;

-- Do not move a template after it has a saved version or is selected by a point-of-sale rule.
DROP TRIGGER IF EXISTS trg_card_template_scope_guard;
CREATE TRIGGER trg_card_template_scope_guard
BEFORE UPDATE OF package_id,point_of_sale_id,scope_kind ON card_templates
FOR EACH ROW
WHEN (NEW.package_id<>OLD.package_id
      OR NEW.point_of_sale_id IS NOT OLD.point_of_sale_id
      OR NEW.scope_kind<>OLD.scope_kind)
 AND (
   EXISTS(SELECT 1 FROM card_template_versions v WHERE v.template_id=OLD.id)
   OR EXISTS(SELECT 1 FROM point_of_sale_packages p WHERE p.template_id_override=OLD.id)
 )
BEGIN
  SELECT RAISE(ABORT,'a referenced template cannot be moved between package or point-of-sale scopes');
END;

CREATE TRIGGER IF NOT EXISTS trg_active_global_default_template_insert
BEFORE INSERT ON card_templates
FOR EACH ROW
WHEN NEW.is_default=1 AND (NEW.scope_kind<>'global' OR NEW.active<>1)
BEGIN
  SELECT RAISE(ABORT,'the default template must be an active global template');
END;
CREATE TRIGGER IF NOT EXISTS trg_active_global_default_template_update
BEFORE UPDATE OF is_default,active,scope_kind,point_of_sale_id ON card_templates
FOR EACH ROW
WHEN NEW.is_default=1 AND (NEW.scope_kind<>'global' OR NEW.active<>1 OR NEW.point_of_sale_id IS NOT NULL)
BEGIN
  SELECT RAISE(ABORT,'the default template must be an active global template');
END;

CREATE TRIGGER IF NOT EXISTS trg_print_job_scope_insert
BEFORE INSERT ON print_jobs
FOR EACH ROW
WHEN NOT EXISTS (
  SELECT 1
  FROM voucher_batches b
  JOIN card_template_versions v ON v.id=NEW.template_version_id
  JOIN card_templates t ON t.id=v.template_id
  WHERE b.id=NEW.batch_id
    AND (b.template_version_id IS NULL OR b.template_version_id=NEW.template_version_id)
    AND (b.package_id IS NULL OR t.package_id=b.package_id)
    AND (t.scope_kind='global' OR t.point_of_sale_id=b.point_of_sale_id)
)
BEGIN
  SELECT RAISE(ABORT,'print job template version must match its batch package and point-of-sale scope');
END;

CREATE TRIGGER IF NOT EXISTS trg_print_job_scope_update
BEFORE UPDATE OF batch_id,template_version_id ON print_jobs
FOR EACH ROW
WHEN NOT EXISTS (
  SELECT 1
  FROM voucher_batches b
  JOIN card_template_versions v ON v.id=NEW.template_version_id
  JOIN card_templates t ON t.id=v.template_id
  WHERE b.id=NEW.batch_id
    AND (b.template_version_id IS NULL OR b.template_version_id=NEW.template_version_id)
    AND (b.package_id IS NULL OR t.package_id=b.package_id)
    AND (t.scope_kind='global' OR t.point_of_sale_id=b.point_of_sale_id)
)
BEGIN
  SELECT RAISE(ABORT,'print job template version must match its batch package and point-of-sale scope');
END;

CREATE TRIGGER IF NOT EXISTS trg_voucher_batch_package_insert
BEFORE INSERT ON vouchers
FOR EACH ROW
WHEN EXISTS (
  SELECT 1 FROM voucher_batches b
  WHERE b.id=NEW.batch_id AND b.package_id IS NOT NULL
    AND (NEW.package_id IS NOT b.package_id OR NEW.point_of_sale_id IS NOT b.point_of_sale_id)
)
BEGIN
  SELECT RAISE(ABORT,'new voucher package and initial point of sale must match its batch');
END;

CREATE TRIGGER IF NOT EXISTS trg_stock_movement_package_insert
BEFORE INSERT ON voucher_stock_movements
FOR EACH ROW
WHEN EXISTS (
  SELECT 1 FROM voucher_batches b
  WHERE b.id=NEW.batch_id AND b.package_id IS NOT NEW.package_id
)
BEGIN
  SELECT RAISE(ABORT,'stock movement package must match its voucher batch');
END;
CREATE TRIGGER IF NOT EXISTS trg_stock_movement_package_update
BEFORE UPDATE OF batch_id,package_id ON voucher_stock_movements
FOR EACH ROW
WHEN EXISTS (
  SELECT 1 FROM voucher_batches b
  WHERE b.id=NEW.batch_id AND b.package_id IS NOT NEW.package_id
)
BEGIN
  SELECT RAISE(ABORT,'stock movement package must match its voucher batch');
END;

-- Inventory movements are an append-only ledger. Corrections should be new adjustment events.
CREATE TRIGGER IF NOT EXISTS trg_stock_movement_no_update
BEFORE UPDATE ON voucher_stock_movements
FOR EACH ROW
BEGIN
  SELECT RAISE(ABORT,'stock movements are append-only; record a correcting movement instead');
END;
CREATE TRIGGER IF NOT EXISTS trg_stock_movement_no_delete
BEFORE DELETE ON voucher_stock_movements
FOR EACH ROW
BEGIN
  SELECT RAISE(ABORT,'stock movements are append-only; record a correcting movement instead');
END;

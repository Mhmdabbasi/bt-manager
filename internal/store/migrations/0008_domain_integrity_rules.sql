-- Cross-table domain invariants that SQLite CHECK constraints cannot express.
-- These keep profile/limitation/package/template relationships coherent regardless of caller.

CREATE TRIGGER IF NOT EXISTS trg_um_profile_limitation_insert
BEFORE INSERT ON um_profile_limitations
FOR EACH ROW
WHEN NOT EXISTS (
  SELECT 1
  FROM service_profiles p
  JOIN um_limitations l ON l.id=NEW.limitation_id
  WHERE p.id=NEW.profile_id
    AND p.source_system='usermanager'
    AND p.router_id=l.router_id
)
BEGIN
  SELECT RAISE(ABORT,'User Manager profile and limitation must belong to the same router');
END;

CREATE TRIGGER IF NOT EXISTS trg_um_profile_limitation_update
BEFORE UPDATE OF profile_id,limitation_id ON um_profile_limitations
FOR EACH ROW
WHEN NOT EXISTS (
  SELECT 1
  FROM service_profiles p
  JOIN um_limitations l ON l.id=NEW.limitation_id
  WHERE p.id=NEW.profile_id
    AND p.source_system='usermanager'
    AND p.router_id=l.router_id
)
BEGIN
  SELECT RAISE(ABORT,'User Manager profile and limitation must belong to the same router');
END;

CREATE TRIGGER IF NOT EXISTS trg_card_package_profile_insert
BEFORE INSERT ON card_packages
FOR EACH ROW
WHEN NOT EXISTS (
  SELECT 1 FROM service_profiles p
  WHERE p.id=NEW.service_profile_id AND p.source_system=NEW.service_system
)
BEGIN
  SELECT RAISE(ABORT,'card package service system must match the selected profile');
END;

CREATE TRIGGER IF NOT EXISTS trg_card_package_profile_update
BEFORE UPDATE OF service_system,service_profile_id ON card_packages
FOR EACH ROW
WHEN NOT EXISTS (
  SELECT 1 FROM service_profiles p
  WHERE p.id=NEW.service_profile_id AND p.source_system=NEW.service_system
)
BEGIN
  SELECT RAISE(ABORT,'card package service system must match the selected profile');
END;

CREATE TRIGGER IF NOT EXISTS trg_pos_package_template_insert
BEFORE INSERT ON point_of_sale_packages
FOR EACH ROW
WHEN NEW.template_id_override IS NOT NULL
 AND NOT EXISTS (
   SELECT 1 FROM card_templates t
   WHERE t.id=NEW.template_id_override
     AND t.package_id=NEW.package_id
     AND (t.scope_kind='global' OR t.point_of_sale_id=NEW.point_of_sale_id)
 )
BEGIN
  SELECT RAISE(ABORT,'point of sale template override must match the selected package and scope');
END;

CREATE TRIGGER IF NOT EXISTS trg_pos_package_template_update
BEFORE UPDATE OF point_of_sale_id,package_id,template_id_override ON point_of_sale_packages
FOR EACH ROW
WHEN NEW.template_id_override IS NOT NULL
 AND NOT EXISTS (
   SELECT 1 FROM card_templates t
   WHERE t.id=NEW.template_id_override
     AND t.package_id=NEW.package_id
     AND (t.scope_kind='global' OR t.point_of_sale_id=NEW.point_of_sale_id)
 )
BEGIN
  SELECT RAISE(ABORT,'point of sale template override must match the selected package and scope');
END;

CREATE TRIGGER IF NOT EXISTS trg_batch_package_system_insert
BEFORE INSERT ON voucher_batches
FOR EACH ROW
WHEN NEW.package_id IS NOT NULL
 AND NOT EXISTS (
   SELECT 1 FROM card_packages p
   WHERE p.id=NEW.package_id AND p.service_system=NEW.service_system_snapshot
 )
BEGIN
  SELECT RAISE(ABORT,'batch service system snapshot must match its card package');
END;

CREATE TRIGGER IF NOT EXISTS trg_batch_template_scope_insert
BEFORE INSERT ON voucher_batches
FOR EACH ROW
WHEN NEW.template_version_id IS NOT NULL
 AND NOT EXISTS (
   SELECT 1
   FROM card_template_versions v
   JOIN card_templates t ON t.id=v.template_id
   WHERE v.id=NEW.template_version_id
     AND (NEW.package_id IS NULL OR t.package_id=NEW.package_id)
     AND (t.scope_kind='global' OR t.point_of_sale_id=NEW.point_of_sale_id)
 )
BEGIN
  SELECT RAISE(ABORT,'batch template version must match the card package and point of sale');
END;

CREATE TRIGGER IF NOT EXISTS trg_batch_package_system_update
BEFORE UPDATE OF package_id,service_system_snapshot ON voucher_batches
FOR EACH ROW
WHEN NEW.package_id IS NOT NULL
 AND NOT EXISTS (
   SELECT 1 FROM card_packages p
   WHERE p.id=NEW.package_id AND p.service_system=NEW.service_system_snapshot
 )
BEGIN
  SELECT RAISE(ABORT,'batch service system snapshot must match its card package');
END;

CREATE TRIGGER IF NOT EXISTS trg_batch_template_scope_update
BEFORE UPDATE OF template_version_id,package_id,point_of_sale_id ON voucher_batches
FOR EACH ROW
WHEN NEW.template_version_id IS NOT NULL
 AND NOT EXISTS (
   SELECT 1
   FROM card_template_versions v
   JOIN card_templates t ON t.id=v.template_id
   WHERE v.id=NEW.template_version_id
     AND (NEW.package_id IS NULL OR t.package_id=NEW.package_id)
     AND (t.scope_kind='global' OR t.point_of_sale_id=NEW.point_of_sale_id)
 )
BEGIN
  SELECT RAISE(ABORT,'batch template version must match the card package and point of sale');
END;

-- Never edit a version already referenced by a batch; save a new version instead.
CREATE TRIGGER IF NOT EXISTS trg_card_template_versions_immutable_update
BEFORE UPDATE ON card_template_versions
FOR EACH ROW
BEGIN
  SELECT RAISE(ABORT,'card template versions are immutable; create a new version');
END;

CREATE TRIGGER IF NOT EXISTS trg_card_template_versions_immutable_delete
BEFORE DELETE ON card_template_versions
FOR EACH ROW
BEGIN
  SELECT RAISE(ABORT,'card template versions are immutable; archive the parent template instead');
END;

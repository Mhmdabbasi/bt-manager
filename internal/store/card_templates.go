package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrCardTemplateVersionNotFound = errors.New("card template version not found")

// ResolveCardTemplateVersion resolves only durable, active template versions.
// A point-of-sale override wins; otherwise the active global default is used.
func (db *DB) ResolveCardTemplateVersion(ctx context.Context, packageID, pointOfSaleID, templateOverrideID int64) (int64, bool, error) {
	if packageID < 1 || pointOfSaleID < 0 || templateOverrideID < 0 {
		return 0, false, ErrInvalidCardPackage
	}
	var versionID int64
	var err error
	if templateOverrideID > 0 {
		err = db.QueryRowContext(ctx, `SELECT v.id
			FROM card_template_versions v
			JOIN card_templates t ON t.id=v.template_id
			WHERE t.id=? AND t.package_id=? AND t.active=1
				AND (t.scope_kind='global' OR (t.scope_kind='point_of_sale' AND t.point_of_sale_id=?))
			ORDER BY v.version_number DESC LIMIT 1`,
			templateOverrideID, packageID, pointOfSaleID).Scan(&versionID)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, ErrCardTemplateVersionNotFound
		}
		if err != nil {
			return 0, false, fmt.Errorf("resolve point-of-sale template override: %w", err)
		}
		return versionID, true, nil
	}
	err = db.QueryRowContext(ctx, `SELECT v.id
		FROM card_template_versions v
		JOIN card_templates t ON t.id=v.template_id
		WHERE t.package_id=? AND t.scope_kind='global' AND t.is_default=1 AND t.active=1
		ORDER BY v.version_number DESC LIMIT 1`, packageID).Scan(&versionID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("resolve global default card template: %w", err)
	}
	return versionID, true, nil
}


// GetCardTemplateVersion returns the immutable design attached to a specific
// version. It intentionally does not require the parent template to remain active:
// previously generated batches must remain printable after a template is retired.
func (db *DB) GetCardTemplateVersion(ctx context.Context, versionID int64) (CardTemplateRecord, error) {
	if versionID < 1 {
		return CardTemplateRecord{}, ErrCardTemplateVersionNotFound
	}
	var item CardTemplateRecord
	var posID, assetID sql.NullInt64
	var design, page string
	err := db.QueryRowContext(ctx, `SELECT t.id,t.package_id,t.point_of_sale_id,t.scope_kind,t.name,t.is_default,t.active,
		v.id,v.version_number,v.card_width_mm,v.card_height_mm,v.design_json,v.page_layout_json,v.background_asset_id,
		t.created_at,t.updated_at
		FROM card_template_versions v JOIN card_templates t ON t.id=v.template_id
		WHERE v.id=?`, versionID).Scan(
		&item.ID,&item.PackageID,&posID,&item.ScopeKind,&item.Name,&item.IsDefault,&item.Active,
		&item.LatestVersionID,&item.VersionNumber,&item.CardWidthMM,&item.CardHeightMM,&design,&page,&assetID,
		&item.CreatedAt,&item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return CardTemplateRecord{}, ErrCardTemplateVersionNotFound
	}
	if err != nil {
		return CardTemplateRecord{}, fmt.Errorf("get card template version: %w", err)
	}
	if posID.Valid { v:=posID.Int64; item.PointOfSaleID=&v }
	if assetID.Valid { v:=assetID.Int64; item.BackgroundAssetID=&v }
	item.DesignJSON=json.RawMessage(design)
	item.PageLayoutJSON=json.RawMessage(page)
	return item,nil
}

// ResolveCardTemplateVersionByID validates an explicitly selected active template
// against the requested package and point-of-sale scope, then pins its latest version.
func (db *DB) ResolveCardTemplateVersionByID(ctx context.Context, packageID, pointOfSaleID, templateID int64) (int64, error) {
	if packageID < 1 || pointOfSaleID < 0 || templateID < 1 {
		return 0, ErrCardTemplateVersionNotFound
	}
	var versionID int64
	err := db.QueryRowContext(ctx, `SELECT v.id
		FROM card_template_versions v JOIN card_templates t ON t.id=v.template_id
		WHERE t.id=? AND t.package_id=? AND t.active=1
			AND (t.scope_kind='global' OR (t.scope_kind='point_of_sale' AND t.point_of_sale_id=?))
		ORDER BY v.version_number DESC LIMIT 1`, templateID, packageID, pointOfSaleID).Scan(&versionID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrCardTemplateVersionNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("resolve selected card template: %w", err)
	}
	return versionID,nil
}

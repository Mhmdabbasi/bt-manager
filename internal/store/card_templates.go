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

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var (
	ErrCardPackageNotFound                 = errors.New("card package not found")
	ErrInvalidCardPackage                  = errors.New("invalid card package")
	ErrCardPackageNameExists               = errors.New("card package name already exists")
	ErrCardPackageServiceProfileNotFound   = errors.New("card package service profile not found")
	ErrCardPackageServiceProfileUnavailable = errors.New("card package service profile is not present on the router")
)

const MaxCardPackagePriceMinor int64 = 1_000_000_000_000
const MaxCardPackageSortOrder = 1_000_000

type CardPackage struct {
	ID                   int64          `json:"id"`
	Name                 string         `json:"name"`
	ServiceSystem        string         `json:"serviceSystem"`
	ServiceProfileID     int64          `json:"serviceProfileId"`
	ServiceProfileName   string         `json:"serviceProfileName"`
	ServiceProfilePresent bool          `json:"serviceProfilePresent"`
	RouterID             int64          `json:"routerId"`
	RouterAddress        string         `json:"routerAddress"`
	RouterIdentity       string         `json:"routerIdentity"`
	Description          string         `json:"description"`
	UserPriceMinor       int64          `json:"userPriceMinor"`
	DealerPriceMinor     int64          `json:"dealerPriceMinor"`
	DefaultPrefix        string         `json:"defaultPrefix"`
	GenerationConfig     map[string]any `json:"generationConfig"`
	Active               bool           `json:"active"`
	SortOrder            int            `json:"sortOrder"`
	CreatedAt            string         `json:"createdAt"`
	UpdatedAt            string         `json:"updatedAt"`
}

type CardPackageInput struct {
	Name             string
	ServiceSystem    string
	ServiceProfileID int64
	Description      string
	UserPriceMinor   int64
	DealerPriceMinor int64
	DefaultPrefix    string
	Active           bool
	SortOrder        int
}

type cardPackageQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

const cardPackageSelect = `SELECT
	cp.id,cp.name,cp.service_system,cp.service_profile_id,sp.name,sp.is_present,
	sp.router_id,r.address,r.router_identity,cp.description,cp.user_price_minor,
	cp.dealer_price_minor,cp.default_prefix,cp.generation_config_json,cp.active,
	cp.sort_order,cp.created_at,cp.updated_at
	FROM card_packages cp
	JOIN service_profiles sp ON sp.id=cp.service_profile_id
	JOIN routers r ON r.id=sp.router_id`

func (db *DB) CreateCardPackage(ctx context.Context, input CardPackageInput) (CardPackage, error) {
	input, err := normalizeCardPackage(input)
	if err != nil {
		return CardPackage{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return CardPackage{}, err
	}
	defer tx.Rollback()

	profilePresent, err := cardPackageProfilePresent(ctx, tx, input.ServiceProfileID, input.ServiceSystem)
	if err != nil {
		return CardPackage{}, err
	}
	if !profilePresent {
		return CardPackage{}, ErrCardPackageServiceProfileUnavailable
	}
	if err := ensureUniqueCardPackageName(ctx, tx, input.Name, 0); err != nil {
		return CardPackage{}, err
	}

	result, err := tx.ExecContext(ctx, `INSERT INTO card_packages
		(name,service_system,service_profile_id,description,user_price_minor,dealer_price_minor,default_prefix,active,sort_order)
		VALUES(?,?,?,?,?,?,?,?,?)`,
		input.Name, input.ServiceSystem, input.ServiceProfileID, input.Description,
		input.UserPriceMinor, input.DealerPriceMinor, input.DefaultPrefix, boolInt(input.Active), input.SortOrder)
	if err != nil {
		return CardPackage{}, fmt.Errorf("create card package: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return CardPackage{}, err
	}
	if err := tx.Commit(); err != nil {
		return CardPackage{}, err
	}
	return db.GetCardPackage(ctx, id)
}

func (db *DB) UpdateCardPackage(ctx context.Context, id int64, input CardPackageInput) (CardPackage, error) {
	if id < 1 {
		return CardPackage{}, ErrCardPackageNotFound
	}
	input, err := normalizeCardPackage(input)
	if err != nil {
		return CardPackage{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return CardPackage{}, err
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM card_packages WHERE id=?", id).Scan(&exists); err != nil {
		return CardPackage{}, err
	}
	if exists == 0 {
		return CardPackage{}, ErrCardPackageNotFound
	}
	profilePresent, err := cardPackageProfilePresent(ctx, tx, input.ServiceProfileID, input.ServiceSystem)
	if err != nil {
		return CardPackage{}, err
	}
	if input.Active && !profilePresent {
		return CardPackage{}, ErrCardPackageServiceProfileUnavailable
	}
	if err := ensureUniqueCardPackageName(ctx, tx, input.Name, id); err != nil {
		return CardPackage{}, err
	}

	_, err = tx.ExecContext(ctx, `UPDATE card_packages SET
		name=?,service_system=?,service_profile_id=?,description=?,user_price_minor=?,
		dealer_price_minor=?,default_prefix=?,active=?,sort_order=?,
		updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?`,
		input.Name, input.ServiceSystem, input.ServiceProfileID, input.Description,
		input.UserPriceMinor, input.DealerPriceMinor, input.DefaultPrefix,
		boolInt(input.Active), input.SortOrder, id)
	if err != nil {
		return CardPackage{}, fmt.Errorf("update card package: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return CardPackage{}, err
	}
	return db.GetCardPackage(ctx, id)
}

func (db *DB) GetCardPackage(ctx context.Context, id int64) (CardPackage, error) {
	if id < 1 {
		return CardPackage{}, ErrCardPackageNotFound
	}
	pkg, err := scanCardPackage(db.QueryRowContext(ctx, cardPackageSelect+" WHERE cp.id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return CardPackage{}, ErrCardPackageNotFound
	}
	return pkg, err
}

func (db *DB) ListCardPackages(ctx context.Context, includeInactive bool, serviceSystem string) ([]CardPackage, error) {
	serviceSystem = strings.ToLower(strings.TrimSpace(serviceSystem))
	if serviceSystem != "" && serviceSystem != "hotspot" && serviceSystem != "usermanager" {
		return nil, ErrInvalidCardPackage
	}
	query := cardPackageSelect
	conditions := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if !includeInactive {
		conditions = append(conditions, "cp.active=1")
	}
	if serviceSystem != "" {
		conditions = append(conditions, "cp.service_system=?")
		args = append(args, serviceSystem)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY cp.sort_order,cp.name COLLATE NOCASE,cp.id"

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	packages := make([]CardPackage, 0)
	for rows.Next() {
		pkg, err := scanCardPackage(rows)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return packages, nil
}

func cardPackageProfilePresent(ctx context.Context, q cardPackageQueryer, profileID int64, serviceSystem string) (bool, error) {
	if profileID < 1 {
		return false, ErrInvalidCardPackage
	}
	var actualSystem string
	var present int
	err := q.QueryRowContext(ctx, "SELECT source_system,is_present FROM service_profiles WHERE id=?", profileID).
		Scan(&actualSystem, &present)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrCardPackageServiceProfileNotFound
	}
	if err != nil {
		return false, err
	}
	if actualSystem != serviceSystem {
		return false, ErrInvalidCardPackage
	}
	return present == 1, nil
}

func ensureUniqueCardPackageName(ctx context.Context, q cardPackageQueryer, name string, exceptID int64) error {
	var exists int
	err := q.QueryRowContext(ctx, "SELECT count(*) FROM card_packages WHERE name=? COLLATE NOCASE AND id<>?", name, exceptID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return ErrCardPackageNameExists
	}
	return nil
}

type cardPackageRowScanner interface {
	Scan(dest ...any) error
}

func scanCardPackage(row cardPackageRowScanner) (CardPackage, error) {
	var pkg CardPackage
	var present, active int
	var generationConfigJSON string
	err := row.Scan(
		&pkg.ID, &pkg.Name, &pkg.ServiceSystem, &pkg.ServiceProfileID, &pkg.ServiceProfileName, &present,
		&pkg.RouterID, &pkg.RouterAddress, &pkg.RouterIdentity, &pkg.Description, &pkg.UserPriceMinor,
		&pkg.DealerPriceMinor, &pkg.DefaultPrefix, &generationConfigJSON, &active, &pkg.SortOrder,
		&pkg.CreatedAt, &pkg.UpdatedAt,
	)
	if err != nil {
		return CardPackage{}, err
	}
	if err := json.Unmarshal([]byte(generationConfigJSON), &pkg.GenerationConfig); err != nil {
		return CardPackage{}, fmt.Errorf("decode card package generation config: %w", err)
	}
	if pkg.GenerationConfig == nil {
		pkg.GenerationConfig = map[string]any{}
	}
	pkg.ServiceProfilePresent = present == 1
	pkg.Active = active == 1
	return pkg, nil
}

func normalizeCardPackage(input CardPackageInput) (CardPackageInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.ServiceSystem = strings.ToLower(strings.TrimSpace(input.ServiceSystem))
	input.Description = strings.TrimSpace(input.Description)
	input.DefaultPrefix = strings.TrimSpace(input.DefaultPrefix)

	if input.Name == "" || len([]rune(input.Name)) > 80 ||
		(input.ServiceSystem != "hotspot" && input.ServiceSystem != "usermanager") ||
		input.ServiceProfileID < 1 || len([]rune(input.Description)) > 1000 ||
		input.UserPriceMinor < 1 || input.UserPriceMinor > MaxCardPackagePriceMinor ||
		input.DealerPriceMinor < 0 || input.DealerPriceMinor > input.UserPriceMinor ||
		len(input.DefaultPrefix) > 32 || !validCardPackagePrefix(input.DefaultPrefix) ||
		input.SortOrder < 0 || input.SortOrder > MaxCardPackageSortOrder ||
		hasCardPackageControlCharacters(input.Name) || hasCardPackageControlCharacters(input.Description) {
		return CardPackageInput{}, ErrInvalidCardPackage
	}
	return input, nil
}

func validCardPackagePrefix(value string) bool {
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func hasCardPackageControlCharacters(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

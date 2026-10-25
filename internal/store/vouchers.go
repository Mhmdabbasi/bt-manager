package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type VoucherBatch struct {
	ID              int64  `json:"id"`
	Reference       string `json:"reference"`
	Prefix          string `json:"prefix"`
	ProfileName     string `json:"profileName"`
	ServerName      string `json:"serverName"`
	LimitUptime     string `json:"limitUptime"`
	UsernameLength  int    `json:"usernameLength"`
	UsernameMode    string `json:"usernameMode"`
	PasswordMode    string `json:"passwordMode"`
	PasswordLength  int    `json:"passwordLength"`
	PasswordCharset string `json:"passwordCharset"`
	RequestedCount  int    `json:"requestedCount"`
	CreatedCount    int    `json:"createdCount"`
	FailedCount     int    `json:"failedCount"`
	Status          string `json:"status"`
	ErrorMessage           string            `json:"errorMessage,omitempty"`
	RouterID               *int64            `json:"routerId,omitempty"`
	PackageID              *int64            `json:"packageId,omitempty"`
	PointOfSaleID          *int64            `json:"pointOfSaleId,omitempty"`
	TemplateVersionID      *int64            `json:"templateVersionId,omitempty"`
	ServiceSystemSnapshot  string            `json:"serviceSystemSnapshot"`
	PackageNameSnapshot    string            `json:"packageNameSnapshot,omitempty"`
	UserPriceMinorSnapshot *int64            `json:"userPriceMinorSnapshot,omitempty"`
	DealerPriceMinorSnapshot *int64          `json:"dealerPriceMinorSnapshot,omitempty"`
	GenerationConfig       map[string]any    `json:"generationConfig,omitempty"`
	ServiceProfileSnapshot map[string]string `json:"serviceProfileSnapshot,omitempty"`
	LimitationSnapshot     []map[string]any  `json:"limitationSnapshot,omitempty"`
	CreatedAt              string            `json:"createdAt"`
}

type Voucher struct {
	ID          int64  `json:"id"`
	BatchID     int64  `json:"batchId"`
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"`
	ProfileName string `json:"profileName"`
	ServerName  string `json:"serverName"`
	LimitUptime string `json:"limitUptime"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
}

type NewVoucherBatch struct {
	Reference       string
	Prefix          string
	ProfileName     string
	ServerName      string
	LimitUptime     string
	UsernameLength  int
	UsernameMode    string
	PasswordMode    string
	PasswordLength  int
	PasswordCharset string
	RequestedCount  int
	RouterID                 int64
	PackageID                int64
	PointOfSaleID            int64
	TemplateVersionID        int64
	ServiceSystemSnapshot    string
	PackageNameSnapshot      string
	UserPriceMinorSnapshot   *int64
	DealerPriceMinorSnapshot *int64
	GenerationConfigJSON     string
	ServiceProfileSnapshotJSON string
	LimitationSnapshotJSON     string
}

var ErrVoucherBatchNotFound = errors.New("voucher batch not found")

func (db *DB) BeginVoucherBatch(ctx context.Context, batch NewVoucherBatch) (VoucherBatch, error) {
	batch.Reference = strings.TrimSpace(batch.Reference)
	batch.Prefix = strings.TrimSpace(batch.Prefix)
	batch.ProfileName = strings.TrimSpace(batch.ProfileName)
	batch.ServerName = strings.TrimSpace(batch.ServerName)
	batch.LimitUptime = strings.TrimSpace(batch.LimitUptime)
	if batch.UsernameLength == 0 { batch.UsernameLength = 8 }
	if batch.UsernameMode == "" { batch.UsernameMode = "alphanumeric" }
	if batch.PasswordMode == "" { batch.PasswordMode = "none" }
	if batch.PasswordLength == 0 { batch.PasswordLength = 4 }
	if batch.PasswordCharset == "" { batch.PasswordCharset = "alphanumeric" }
	if batch.Reference == "" || batch.ProfileName == "" || batch.RequestedCount < 1 || batch.RequestedCount > 200 {
		return VoucherBatch{}, errors.New("invalid voucher batch")
	}
	if batch.ServerName == "" {
		batch.ServerName = "all"
	}
	if batch.ServiceSystemSnapshot == "" {
		batch.ServiceSystemSnapshot = "hotspot"
	}
	if batch.ServiceSystemSnapshot != "hotspot" && batch.ServiceSystemSnapshot != "usermanager" {
		return VoucherBatch{}, errors.New("invalid voucher batch service system")
	}
	if batch.PointOfSaleID > 0 && batch.PackageID < 1 {
		return VoucherBatch{}, errors.New("point-of-sale batches require a card package")
	}
	if batch.PackageID > 0 && (batch.RouterID < 1 || batch.PackageNameSnapshot == "" ||
		batch.UserPriceMinorSnapshot == nil || batch.DealerPriceMinorSnapshot == nil ||
		*batch.UserPriceMinorSnapshot < 1 || *batch.DealerPriceMinorSnapshot < 0 ||
		*batch.DealerPriceMinorSnapshot > *batch.UserPriceMinorSnapshot) {
		return VoucherBatch{}, errors.New("incomplete package voucher batch snapshot")
	}
	batch.GenerationConfigJSON = defaultJSON(batch.GenerationConfigJSON, "{}")
	batch.ServiceProfileSnapshotJSON = defaultJSON(batch.ServiceProfileSnapshotJSON, "{}")
	batch.LimitationSnapshotJSON = defaultJSON(batch.LimitationSnapshotJSON, "[]")
	if !json.Valid([]byte(batch.GenerationConfigJSON)) || !json.Valid([]byte(batch.ServiceProfileSnapshotJSON)) ||
		!json.Valid([]byte(batch.LimitationSnapshotJSON)) {
		return VoucherBatch{}, errors.New("invalid voucher batch snapshot JSON")
	}
	result, err := db.ExecContext(ctx, `INSERT INTO voucher_batches
		(reference,prefix,profile_name,server_name,limit_uptime,username_length,username_mode,
		password_mode,password_length,password_charset,requested_count,status,router_id,package_id,
		point_of_sale_id,template_version_id,service_system_snapshot,package_name_snapshot,
		user_price_minor_snapshot,dealer_price_minor_snapshot,generation_config_json,
		service_profile_snapshot_json,limitation_snapshot_json)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,'running',?,?,?,?,?,?,?,?,?,?,?)`,
		batch.Reference, batch.Prefix, batch.ProfileName, batch.ServerName,
		batch.LimitUptime, batch.UsernameLength, batch.UsernameMode, batch.PasswordMode,
		batch.PasswordLength, batch.PasswordCharset, batch.RequestedCount,
		nullableID(batch.RouterID), nullableID(batch.PackageID), nullableID(batch.PointOfSaleID),
		nullableID(batch.TemplateVersionID), batch.ServiceSystemSnapshot, batch.PackageNameSnapshot,
		nullableInt64(batch.UserPriceMinorSnapshot), nullableInt64(batch.DealerPriceMinorSnapshot),
		batch.GenerationConfigJSON, batch.ServiceProfileSnapshotJSON, batch.LimitationSnapshotJSON)
	if err != nil {
		return VoucherBatch{}, fmt.Errorf("create voucher batch: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return VoucherBatch{}, err
	}
	return db.GetVoucherBatch(ctx, id)
}

func (db *DB) AddVoucher(ctx context.Context, batchID int64, username string, passwords ...string) (Voucher, error) {
	username = strings.TrimSpace(username)
	password := ""
	if len(passwords) > 0 { password = passwords[0] }
	if batchID <= 0 || username == "" {
		return Voucher{}, errors.New("invalid voucher")
	}
	// Read the batch metadata before adding a voucher so the stored card
	// always reflects the profile/server settings used for this batch.
	var profile, server, uptime, status string
	var packageID, pointOfSaleID sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT profile_name,server_name,limit_uptime,status,package_id,point_of_sale_id FROM voucher_batches WHERE id=?`, batchID).
		Scan(&profile, &server, &uptime, &status, &packageID, &pointOfSaleID); err != nil {
		return Voucher{}, err
	}
	if status != "running" {
		return Voucher{}, errors.New("voucher batch is not running")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO vouchers(batch_id,username,password,profile_name,server_name,limit_uptime,package_id,point_of_sale_id) VALUES(?,?,?,?,?,?,?,?)`,
		batchID, username, password, profile, server, uptime, nullableSQLInt64(packageID), nullableSQLInt64(pointOfSaleID)); err != nil {
		return Voucher{}, fmt.Errorf("save voucher %s: %w", username, err)
	}
	return db.getVoucherByUsername(ctx, username)
}

func (db *DB) FinishVoucherBatch(ctx context.Context, id int64, reportedFailed int, message string) (VoucherBatch, error) {
	_ = reportedFailed // The database inventory is authoritative when calculating failures.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return VoucherBatch{}, err
	}
	defer tx.Rollback()

	var created, requested int
	var packageID, pointOfSaleID sql.NullInt64
	var reference, currentStatus string
	err = tx.QueryRowContext(ctx, `SELECT requested_count,package_id,point_of_sale_id,reference,status
		FROM voucher_batches WHERE id=?`, id).Scan(&requested, &packageID, &pointOfSaleID, &reference, &currentStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return VoucherBatch{}, ErrVoucherBatchNotFound
	}
	if err != nil {
		return VoucherBatch{}, err
	}
	if currentStatus != "running" {
		return db.GetVoucherBatch(ctx, id)
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM vouchers WHERE batch_id=?`, id).Scan(&created); err != nil {
		return VoucherBatch{}, err
	}
	failed := requested - created
	if failed < 0 {
		failed = 0
	}
	status := "completed"
	if created == 0 {
		status = "failed"
	} else if created < requested {
		status = "partial"
	}
	if _, err := tx.ExecContext(ctx, `UPDATE voucher_batches SET created_count=?,failed_count=?,status=?,error_message=? WHERE id=?`,
		created, failed, status, strings.TrimSpace(message), id); err != nil {
		return VoucherBatch{}, err
	}

	// Package-backed voucher inventory is recorded in the append-only stock ledger.
	// Legacy batches have NULL package_id and retain their existing behavior.
	if packageID.Valid && created > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO voucher_stock_movements
			(reference,movement_type,batch_id,package_id,quantity,note)
			VALUES(?,'issue',?,?,?,'Initial voucher generation')`,
			reference+"-ISSUE", id, packageID.Int64, created); err != nil {
			return VoucherBatch{}, fmt.Errorf("record initial voucher stock: %w", err)
		}
		if pointOfSaleID.Valid {
			if _, err := tx.ExecContext(ctx, `INSERT INTO voucher_stock_movements
				(reference,movement_type,batch_id,package_id,to_point_of_sale_id,quantity,note)
				VALUES(?,'allocate',?,?,?,?, 'Initial allocation to point of sale')`,
				reference+"-ALLOC-"+fmt.Sprint(pointOfSaleID.Int64), id, packageID.Int64, pointOfSaleID.Int64, created); err != nil {
				return VoucherBatch{}, fmt.Errorf("record initial point-of-sale allocation: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return VoucherBatch{}, err
	}
	return db.GetVoucherBatch(ctx, id)
}

const voucherBatchSelect = `SELECT id,reference,prefix,profile_name,server_name,limit_uptime,
	username_length,username_mode,password_mode,password_length,password_charset,requested_count,
	created_count,failed_count,status,error_message,router_id,package_id,point_of_sale_id,template_version_id,
	service_system_snapshot,package_name_snapshot,user_price_minor_snapshot,dealer_price_minor_snapshot,
	generation_config_json,service_profile_snapshot_json,limitation_snapshot_json,created_at
	FROM voucher_batches`

type voucherBatchRowScanner interface {
	Scan(dest ...any) error
}

func scanVoucherBatch(row voucherBatchRowScanner) (VoucherBatch, error) {
	var batch VoucherBatch
	var routerID, packageID, pointOfSaleID, templateVersionID sql.NullInt64
	var userPrice, dealerPrice sql.NullInt64
	var generationJSON, profileJSON, limitationJSON string
	err := row.Scan(
		&batch.ID, &batch.Reference, &batch.Prefix, &batch.ProfileName, &batch.ServerName, &batch.LimitUptime,
		&batch.UsernameLength, &batch.UsernameMode, &batch.PasswordMode, &batch.PasswordLength, &batch.PasswordCharset,
		&batch.RequestedCount, &batch.CreatedCount, &batch.FailedCount, &batch.Status, &batch.ErrorMessage,
		&routerID, &packageID, &pointOfSaleID, &templateVersionID, &batch.ServiceSystemSnapshot, &batch.PackageNameSnapshot,
		&userPrice, &dealerPrice, &generationJSON, &profileJSON, &limitationJSON, &batch.CreatedAt,
	)
	if err != nil {
		return VoucherBatch{}, err
	}
	batch.RouterID = sqlInt64Pointer(routerID)
	batch.PackageID = sqlInt64Pointer(packageID)
	batch.PointOfSaleID = sqlInt64Pointer(pointOfSaleID)
	batch.TemplateVersionID = sqlInt64Pointer(templateVersionID)
	batch.UserPriceMinorSnapshot = sqlInt64Pointer(userPrice)
	batch.DealerPriceMinorSnapshot = sqlInt64Pointer(dealerPrice)
	batch.GenerationConfig = make(map[string]any)
	if err := json.Unmarshal([]byte(defaultJSON(generationJSON, "{}")), &batch.GenerationConfig); err != nil {
		return VoucherBatch{}, fmt.Errorf("decode voucher batch generation snapshot: %w", err)
	}
	batch.ServiceProfileSnapshot = make(map[string]string)
	if err := json.Unmarshal([]byte(defaultJSON(profileJSON, "{}")), &batch.ServiceProfileSnapshot); err != nil {
		return VoucherBatch{}, fmt.Errorf("decode voucher batch profile snapshot: %w", err)
	}
	batch.LimitationSnapshot = make([]map[string]any, 0)
	if err := json.Unmarshal([]byte(defaultJSON(limitationJSON, "[]")), &batch.LimitationSnapshot); err != nil {
		return VoucherBatch{}, fmt.Errorf("decode voucher batch limitation snapshot: %w", err)
	}
	return batch, nil
}

func sqlInt64Pointer(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}

func nullableID(value int64) any {
	if value < 1 {
		return nil
	}
	return value
}

func nullableSQLInt64(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func defaultJSON(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func (db *DB) GetVoucherBatch(ctx context.Context, id int64) (VoucherBatch, error) {
	batch, err := scanVoucherBatch(db.QueryRowContext(ctx, voucherBatchSelect+" WHERE id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return VoucherBatch{}, ErrVoucherBatchNotFound
	}
	return batch, err
}

func (db *DB) ListVoucherBatches(ctx context.Context, limit int) ([]VoucherBatch, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows, err := db.QueryContext(ctx, voucherBatchSelect+" ORDER BY id DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	batches := make([]VoucherBatch, 0)
	for rows.Next() {
		batch, err := scanVoucherBatch(rows)
		if err != nil {
			return nil, err
		}
		batches = append(batches, batch)
	}
	return batches, rows.Err()
}

func (db *DB) ListVouchersByBatch(ctx context.Context, batchID int64) ([]Voucher, error) {
	rows, err := db.QueryContext(ctx, `SELECT id,batch_id,username,password,profile_name,server_name,limit_uptime,status,created_at
		FROM vouchers WHERE batch_id=? ORDER BY id LIMIT 200`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	vouchers := make([]Voucher, 0)
	for rows.Next() {
		var voucher Voucher
		if err := rows.Scan(&voucher.ID, &voucher.BatchID, &voucher.Username, &voucher.Password, &voucher.ProfileName,
			&voucher.ServerName, &voucher.LimitUptime, &voucher.Status, &voucher.CreatedAt); err != nil {
			return nil, err
		}
		vouchers = append(vouchers, voucher)
	}
	return vouchers, rows.Err()
}

func (db *DB) getVoucherByUsername(ctx context.Context, username string) (Voucher, error) {
	var voucher Voucher
	err := db.QueryRowContext(ctx, `SELECT id,batch_id,username,password,profile_name,server_name,limit_uptime,status,created_at
		FROM vouchers WHERE username=?`, username).
		Scan(&voucher.ID, &voucher.BatchID, &voucher.Username, &voucher.Password, &voucher.ProfileName,
			&voucher.ServerName, &voucher.LimitUptime, &voucher.Status, &voucher.CreatedAt)
	return voucher, err
}

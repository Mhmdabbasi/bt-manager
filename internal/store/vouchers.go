package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type VoucherBatch struct {
	ID            int64  `json:"id"`
	Reference     string `json:"reference"`
	Prefix        string `json:"prefix"`
	ProfileName   string `json:"profileName"`
	ServerName    string `json:"serverName"`
	LimitUptime   string `json:"limitUptime"`
	RequestedCount int   `json:"requestedCount"`
	CreatedCount  int    `json:"createdCount"`
	FailedCount   int    `json:"failedCount"`
	Status        string `json:"status"`
	ErrorMessage  string `json:"errorMessage,omitempty"`
	CreatedAt     string `json:"createdAt"`
}

type Voucher struct {
	ID          int64  `json:"id"`
	BatchID     int64  `json:"batchId"`
	Username    string `json:"username"`
	ProfileName string `json:"profileName"`
	ServerName  string `json:"serverName"`
	LimitUptime string `json:"limitUptime"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
}

type NewVoucherBatch struct {
	Reference      string
	Prefix         string
	ProfileName    string
	ServerName     string
	LimitUptime    string
	RequestedCount int
}

var ErrVoucherBatchNotFound = errors.New("voucher batch not found")

func (db *DB) BeginVoucherBatch(ctx context.Context, batch NewVoucherBatch) (VoucherBatch, error) {
	batch.Reference = strings.TrimSpace(batch.Reference)
	batch.Prefix = strings.TrimSpace(batch.Prefix)
	batch.ProfileName = strings.TrimSpace(batch.ProfileName)
	batch.ServerName = strings.TrimSpace(batch.ServerName)
	batch.LimitUptime = strings.TrimSpace(batch.LimitUptime)
	if batch.Reference == "" || batch.ProfileName == "" || batch.RequestedCount < 1 || batch.RequestedCount > 200 {
		return VoucherBatch{}, errors.New("invalid voucher batch")
	}
	if batch.ServerName == "" {
		batch.ServerName = "all"
	}
	result, err := db.ExecContext(ctx, `INSERT INTO voucher_batches
		(reference,prefix,profile_name,server_name,limit_uptime,requested_count,status)
		VALUES(?,?,?,?,?,?,'running')`,
		batch.Reference, batch.Prefix, batch.ProfileName, batch.ServerName,
		batch.LimitUptime, batch.RequestedCount)
	if err != nil {
		return VoucherBatch{}, fmt.Errorf("create voucher batch: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return VoucherBatch{}, err
	}
	return db.GetVoucherBatch(ctx, id)
}

func (db *DB) AddVoucher(ctx context.Context, batchID int64, username string) (Voucher, error) {
	username = strings.TrimSpace(username)
	if batchID <= 0 || username == "" {
		return Voucher{}, errors.New("invalid voucher")
	}
	// Read the batch metadata before adding a voucher so the stored card
	// always reflects the profile/server settings used for this batch.
	var profile, server, uptime, status string
	if err := db.QueryRowContext(ctx, `SELECT profile_name,server_name,limit_uptime,status FROM voucher_batches WHERE id=?`, batchID).
		Scan(&profile, &server, &uptime, &status); err != nil {
		return Voucher{}, err
	}
	if status != "running" {
		return Voucher{}, errors.New("voucher batch is not running")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO vouchers(batch_id,username,profile_name,server_name,limit_uptime) VALUES(?,?,?,?,?)`,
		batchID, username, profile, server, uptime); err != nil {
		return Voucher{}, fmt.Errorf("save voucher %s: %w", username, err)
	}
	return db.getVoucherByUsername(ctx, username)
}

func (db *DB) FinishVoucherBatch(ctx context.Context, id int64, reportedFailed int, message string) (VoucherBatch, error) {
	_ = reportedFailed // The database inventory is authoritative when calculating failures.
	var created, requested int
	if err := db.QueryRowContext(ctx, `SELECT requested_count FROM voucher_batches WHERE id=?`, id).Scan(&requested); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return VoucherBatch{}, ErrVoucherBatchNotFound
		}
		return VoucherBatch{}, err
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM vouchers WHERE batch_id=?`, id).Scan(&created); err != nil {
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
	if _, err := db.ExecContext(ctx, `UPDATE voucher_batches SET created_count=?,failed_count=?,status=?,error_message=? WHERE id=?`,
		created, failed, status, strings.TrimSpace(message), id); err != nil {
		return VoucherBatch{}, err
	}
	return db.GetVoucherBatch(ctx, id)
}

func (db *DB) GetVoucherBatch(ctx context.Context, id int64) (VoucherBatch, error) {
	var batch VoucherBatch
	err := db.QueryRowContext(ctx, `SELECT id,reference,prefix,profile_name,server_name,limit_uptime,requested_count,
		created_count,failed_count,status,error_message,created_at FROM voucher_batches WHERE id=?`, id).
		Scan(&batch.ID, &batch.Reference, &batch.Prefix, &batch.ProfileName, &batch.ServerName,
			&batch.LimitUptime, &batch.RequestedCount, &batch.CreatedCount, &batch.FailedCount,
			&batch.Status, &batch.ErrorMessage, &batch.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return VoucherBatch{}, ErrVoucherBatchNotFound
	}
	return batch, err
}

func (db *DB) ListVoucherBatches(ctx context.Context, limit int) ([]VoucherBatch, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows, err := db.QueryContext(ctx, `SELECT id,reference,prefix,profile_name,server_name,limit_uptime,requested_count,
		created_count,failed_count,status,error_message,created_at FROM voucher_batches ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	batches := make([]VoucherBatch, 0)
	for rows.Next() {
		var batch VoucherBatch
		if err := rows.Scan(&batch.ID, &batch.Reference, &batch.Prefix, &batch.ProfileName, &batch.ServerName,
			&batch.LimitUptime, &batch.RequestedCount, &batch.CreatedCount, &batch.FailedCount,
			&batch.Status, &batch.ErrorMessage, &batch.CreatedAt); err != nil {
			return nil, err
		}
		batches = append(batches, batch)
	}
	return batches, rows.Err()
}

func (db *DB) ListVouchersByBatch(ctx context.Context, batchID int64) ([]Voucher, error) {
	rows, err := db.QueryContext(ctx, `SELECT id,batch_id,username,profile_name,server_name,limit_uptime,status,created_at
		FROM vouchers WHERE batch_id=? ORDER BY id LIMIT 200`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	vouchers := make([]Voucher, 0)
	for rows.Next() {
		var voucher Voucher
		if err := rows.Scan(&voucher.ID, &voucher.BatchID, &voucher.Username, &voucher.ProfileName,
			&voucher.ServerName, &voucher.LimitUptime, &voucher.Status, &voucher.CreatedAt); err != nil {
			return nil, err
		}
		vouchers = append(vouchers, voucher)
	}
	return vouchers, rows.Err()
}

func (db *DB) getVoucherByUsername(ctx context.Context, username string) (Voucher, error) {
	var voucher Voucher
	err := db.QueryRowContext(ctx, `SELECT id,batch_id,username,profile_name,server_name,limit_uptime,status,created_at
		FROM vouchers WHERE username=?`, username).
		Scan(&voucher.ID, &voucher.BatchID, &voucher.Username, &voucher.ProfileName,
			&voucher.ServerName, &voucher.LimitUptime, &voucher.Status, &voucher.CreatedAt)
	return voucher, err
}

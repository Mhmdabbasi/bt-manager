package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrDesignAssetNotFound = errors.New("design asset not found")

type DesignAsset struct {
	ID           int64  `json:"id"`
	AssetKey     string `json:"assetKey"`
	AssetKind    string `json:"assetKind"`
	OriginalName string `json:"originalName"`
	MIMEType     string `json:"mimeType"`
	SizeBytes    int64  `json:"sizeBytes"`
	SHA256       string `json:"sha256"`
	WidthPX      *int   `json:"widthPx,omitempty"`
	HeightPX     *int   `json:"heightPx,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

type DesignAssetInput struct {
	AssetKey     string
	AssetKind    string
	OriginalName string
	MIMEType     string
	SizeBytes    int64
	SHA256       string
	WidthPX      *int
	HeightPX     *int
}

func (db *DB) CreateDesignAsset(ctx context.Context, input DesignAssetInput) (DesignAsset, error) {
	input.AssetKey = strings.TrimSpace(input.AssetKey)
	input.AssetKind = strings.TrimSpace(input.AssetKind)
	input.OriginalName = strings.TrimSpace(input.OriginalName)
	if input.AssetKey == "" || input.OriginalName == "" || input.SizeBytes < 1 ||
		(input.AssetKind != "background" && input.AssetKind != "image") ||
		(input.MIMEType != "image/png" && input.MIMEType != "image/jpeg" && input.MIMEType != "image/webp") {
		return DesignAsset{}, errors.New("invalid design asset")
	}
	result, err := db.ExecContext(ctx, `INSERT INTO design_assets
		(asset_key,asset_kind,original_name,mime_type,size_bytes,sha256,width_px,height_px)
		VALUES(?,?,?,?,?,?,?,?)`, input.AssetKey, input.AssetKind, input.OriginalName, input.MIMEType,
		input.SizeBytes, input.SHA256, input.WidthPX, input.HeightPX)
	if err != nil {
		return DesignAsset{}, fmt.Errorf("insert design asset: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return DesignAsset{}, err
	}
	return db.GetDesignAsset(ctx, id)
}

func (db *DB) GetDesignAsset(ctx context.Context, id int64) (DesignAsset, error) {
	row := db.QueryRowContext(ctx, `SELECT id,asset_key,asset_kind,original_name,mime_type,size_bytes,sha256,width_px,height_px,created_at
		FROM design_assets WHERE id=?`, id)
	var asset DesignAsset
	err := row.Scan(&asset.ID, &asset.AssetKey, &asset.AssetKind, &asset.OriginalName, &asset.MIMEType,
		&asset.SizeBytes, &asset.SHA256, &asset.WidthPX, &asset.HeightPX, &asset.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DesignAsset{}, ErrDesignAssetNotFound
	}
	if err != nil {
		return DesignAsset{}, fmt.Errorf("read design asset: %w", err)
	}
	return asset, nil
}

func (db *DB) GetDesignAssetByKey(ctx context.Context, key string) (DesignAsset, error) {
	row := db.QueryRowContext(ctx, `SELECT id,asset_key,asset_kind,original_name,mime_type,size_bytes,sha256,width_px,height_px,created_at
		FROM design_assets WHERE asset_key=?`, key)
	var asset DesignAsset
	err := row.Scan(&asset.ID, &asset.AssetKey, &asset.AssetKind, &asset.OriginalName, &asset.MIMEType,
		&asset.SizeBytes, &asset.SHA256, &asset.WidthPX, &asset.HeightPX, &asset.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DesignAsset{}, ErrDesignAssetNotFound
	}
	if err != nil {
		return DesignAsset{}, fmt.Errorf("read design asset by key: %w", err)
	}
	return asset, nil
}

func (db *DB) ListDesignAssets(ctx context.Context) ([]DesignAsset, error) {
	rows, err := db.QueryContext(ctx, `SELECT id,asset_key,asset_kind,original_name,mime_type,size_bytes,sha256,width_px,height_px,created_at
		FROM design_assets ORDER BY id DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	assets := make([]DesignAsset, 0)
	for rows.Next() {
		var asset DesignAsset
		if err := rows.Scan(&asset.ID, &asset.AssetKey, &asset.AssetKind, &asset.OriginalName, &asset.MIMEType,
			&asset.SizeBytes, &asset.SHA256, &asset.WidthPX, &asset.HeightPX, &asset.CreatedAt); err != nil { return nil, err }
		assets = append(assets, asset)
	}
	if err := rows.Err(); err != nil { return nil, err }
	return assets, nil
}

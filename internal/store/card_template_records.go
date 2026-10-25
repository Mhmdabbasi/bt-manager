package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrCardTemplateNotFound = errors.New("card template not found")

type CardTemplateRecord struct {
	ID int64 `json:"id"`
	PackageID int64 `json:"packageId"`
	PointOfSaleID *int64 `json:"pointOfSaleId,omitempty"`
	ScopeKind string `json:"scopeKind"`
	Name string `json:"name"`
	IsDefault bool `json:"isDefault"`
	Active bool `json:"active"`
	LatestVersionID *int64 `json:"latestVersionId,omitempty"`
	VersionNumber int `json:"versionNumber"`
	CardWidthMM float64 `json:"cardWidthMm"`
	CardHeightMM float64 `json:"cardHeightMm"`
	DesignJSON json.RawMessage `json:"design"`
	PageLayoutJSON json.RawMessage `json:"pageLayout"`
	BackgroundAssetID *int64 `json:"backgroundAssetId,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type SaveCardTemplateInput struct {
	ID int64
	PackageID int64
	PointOfSaleID int64
	Name string
	IsDefault bool
	CardWidthMM float64
	CardHeightMM float64
	DesignJSON json.RawMessage
	PageLayoutJSON json.RawMessage
	BackgroundAssetID int64
}

func (db *DB) ListCardTemplates(ctx context.Context, packageID int64) ([]CardTemplateRecord, error) {
	query := `SELECT t.id,t.package_id,t.point_of_sale_id,t.scope_kind,t.name,t.is_default,t.active,
		v.id,COALESCE(v.version_number,0),COALESCE(v.card_width_mm,50),COALESCE(v.card_height_mm,19.1),
		COALESCE(v.design_json,'{}'),COALESCE(v.page_layout_json,'{}'),v.background_asset_id,t.created_at,t.updated_at
		FROM card_templates t LEFT JOIN card_template_versions v ON v.id=(
			SELECT v2.id FROM card_template_versions v2 WHERE v2.template_id=t.id ORDER BY v2.version_number DESC LIMIT 1)
		WHERE (?=0 OR t.package_id=?) ORDER BY t.package_id,t.scope_kind,t.point_of_sale_id,t.is_default DESC,t.name COLLATE NOCASE`
	rows, err := db.QueryContext(ctx, query, packageID, packageID)
	if err != nil { return nil, fmt.Errorf("list card templates: %w", err) }
	defer rows.Close()
	templates := make([]CardTemplateRecord,0)
	for rows.Next() {
		var item CardTemplateRecord
		var posID, versionID, assetID sql.NullInt64
		var design,page string
		if err := rows.Scan(&item.ID,&item.PackageID,&posID,&item.ScopeKind,&item.Name,&item.IsDefault,&item.Active,
			&versionID,&item.VersionNumber,&item.CardWidthMM,&item.CardHeightMM,&design,&page,&assetID,&item.CreatedAt,&item.UpdatedAt); err != nil { return nil, err }
		if posID.Valid { v:=posID.Int64; item.PointOfSaleID=&v }
		if versionID.Valid { v:=versionID.Int64; item.LatestVersionID=&v }
		if assetID.Valid { v:=assetID.Int64; item.BackgroundAssetID=&v }
		item.DesignJSON=json.RawMessage(design); item.PageLayoutJSON=json.RawMessage(page)
		templates=append(templates,item)
	}
	if err:=rows.Err();err!=nil{return nil,err}
	return templates,nil
}

func (db *DB) SaveCardTemplateVersion(ctx context.Context, input SaveCardTemplateInput) (CardTemplateRecord,error) {
	input.Name=strings.TrimSpace(input.Name)
	if input.PackageID<1 || input.Name=="" || len(input.Name)>100 || input.CardWidthMM<=0 || input.CardWidthMM>300 ||
		input.CardHeightMM<=0 || input.CardHeightMM>300 { return CardTemplateRecord{}, errors.New("invalid card template") }
	if len(input.DesignJSON)==0 { input.DesignJSON=json.RawMessage(`{}`) }
	if len(input.PageLayoutJSON)==0 { input.PageLayoutJSON=json.RawMessage(`{}`) }
	if !json.Valid(input.DesignJSON)||!json.Valid(input.PageLayoutJSON) { return CardTemplateRecord{},errors.New("invalid template JSON") }
	tx,err:=db.BeginTx(ctx,nil); if err!=nil{return CardTemplateRecord{},err}; defer tx.Rollback()
	scope:="global"; var pos any
	if input.PointOfSaleID>0 { scope="point_of_sale"; pos=input.PointOfSaleID; input.IsDefault=false }
	var templateID int64
	if input.ID>0 {
		if input.IsDefault { if _,err:=tx.ExecContext(ctx,`UPDATE card_templates SET is_default=0,updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE package_id=? AND scope_kind='global' AND id<>?`,input.PackageID,input.ID);err!=nil{return CardTemplateRecord{},err} }
		res,err:=tx.ExecContext(ctx,`UPDATE card_templates SET name=?,is_default=?,updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=? AND package_id=? AND scope_kind=? AND ((point_of_sale_id IS NULL AND ? IS NULL) OR point_of_sale_id=?)`,input.Name,input.IsDefault,input.ID,input.PackageID,scope,pos,pos)
		if err!=nil{return CardTemplateRecord{},fmt.Errorf("update card template: %w",err)}
		n,_:=res.RowsAffected(); if n==0{return CardTemplateRecord{},ErrCardTemplateNotFound}; templateID=input.ID
	} else {
		if input.IsDefault { if _,err:=tx.ExecContext(ctx,`UPDATE card_templates SET is_default=0,updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE package_id=? AND scope_kind='global' AND is_default=1`,input.PackageID);err!=nil{return CardTemplateRecord{},err} }
		res,err:=tx.ExecContext(ctx,`INSERT INTO card_templates(package_id,point_of_sale_id,scope_kind,name,is_default,active) VALUES(?,?,?,?,?,1)`,
			input.PackageID,pos,scope,input.Name,input.IsDefault)
		if err!=nil{return CardTemplateRecord{},fmt.Errorf("create card template: %w",err)}
		templateID,err=res.LastInsertId();if err!=nil{return CardTemplateRecord{},err}
	}
	if input.IsDefault { if _,err:=tx.ExecContext(ctx,`UPDATE card_templates SET is_default=0 WHERE package_id=? AND scope_kind='global' AND id<>?`,input.PackageID,templateID);err!=nil{return CardTemplateRecord{},err}; if _,err:=tx.ExecContext(ctx,`UPDATE card_templates SET is_default=1 WHERE id=?`,templateID);err!=nil{return CardTemplateRecord{},err} }
	if input.PointOfSaleID>0 {
		_,err:=tx.ExecContext(ctx,`INSERT INTO point_of_sale_packages(point_of_sale_id,package_id,is_enabled,template_id_override)
			VALUES(?,?,1,?) ON CONFLICT(point_of_sale_id,package_id) DO UPDATE SET template_id_override=excluded.template_id_override,updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')`,input.PointOfSaleID,input.PackageID,templateID)
		if err!=nil{return CardTemplateRecord{},fmt.Errorf("assign point-of-sale template: %w",err)}
	}
	var next int
	if err:=tx.QueryRowContext(ctx,`SELECT COALESCE(MAX(version_number),0)+1 FROM card_template_versions WHERE template_id=?`,templateID).Scan(&next);err!=nil{return CardTemplateRecord{},err}
	var asset any; if input.BackgroundAssetID>0 { asset=input.BackgroundAssetID }
	if _,err:=tx.ExecContext(ctx,`INSERT INTO card_template_versions(template_id,version_number,card_width_mm,card_height_mm,design_json,page_layout_json,background_asset_id)
		VALUES(?,?,?,?,?,?,?)`,templateID,next,input.CardWidthMM,input.CardHeightMM,string(input.DesignJSON),string(input.PageLayoutJSON),asset);err!=nil{return CardTemplateRecord{},fmt.Errorf("insert card template version: %w",err)}
	if err:=tx.Commit();err!=nil{return CardTemplateRecord{},err}
	items,err:=db.ListCardTemplates(ctx,input.PackageID);if err!=nil{return CardTemplateRecord{},err}
	for _,item:=range items { if item.ID==templateID{return item,nil} }
	return CardTemplateRecord{},ErrCardTemplateNotFound
}

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCardDomainSchemaRelationshipsAndInvariants(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "domain.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	// Same router may expose both HotSpot and User Manager profiles with similar source IDs.
	routerResult, err := db.ExecContext(ctx, `INSERT INTO routers(router_key,address,api_port,router_identity)
		VALUES('router-test-key','192.0.2.1',8728,'test-router')`)
	if err != nil {
		t.Fatal(err)
	}
	routerID, _ := routerResult.LastInsertId()

	hotspotResult, err := db.ExecContext(ctx, `INSERT INTO service_profiles(router_id,source_system,source_id,name,properties_json)
		VALUES(?,'hotspot','*1','1M','{"rate-limit":"1M/1M"}')`, routerID)
	if err != nil {
		t.Fatal(err)
	}
	hotspotProfileID, _ := hotspotResult.LastInsertId()

	umProfileResult, err := db.ExecContext(ctx, `INSERT INTO service_profiles(router_id,source_system,source_id,name,properties_json)
		VALUES(?,'usermanager','*1','UM-200','{"name":"UM-200"}')`, routerID)
	if err != nil {
		t.Fatal(err)
	}
	umProfileID, _ := umProfileResult.LastInsertId()

	limitationResult, err := db.ExecContext(ctx, `INSERT INTO um_limitations(router_id,source_id,name,properties_json)
		VALUES(?,'*7','200MB','{"transfer-limit":"200M"}')`, routerID)
	if err != nil {
		t.Fatal(err)
	}
	limitationID, _ := limitationResult.LastInsertId()
	if _, err := db.ExecContext(ctx, `INSERT INTO um_profile_limitations(profile_id,limitation_id,position)
		VALUES(?,?,0)`, umProfileID, limitationID); err != nil {
		t.Fatalf("valid profile/limitation relationship failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO um_profile_limitations(profile_id,limitation_id)
		VALUES(?,?)`, hotspotProfileID, limitationID); err == nil {
		t.Fatal("HotSpot profile was incorrectly accepted as a User Manager profile")
	}

	packageResult, err := db.ExecContext(ctx, `INSERT INTO card_packages
		(name,service_system,service_profile_id,user_price_minor,dealer_price_minor,default_prefix)
		VALUES('Abu 200','usermanager',?,20000,16000,'AB2-')`, umProfileID)
	if err != nil {
		t.Fatal(err)
	}
	packageID, _ := packageResult.LastInsertId()
	if _, err := db.ExecContext(ctx, `INSERT INTO card_packages
		(name,service_system,service_profile_id,user_price_minor,dealer_price_minor)
		VALUES('Invalid package','usermanager',?,20000,16000)`, hotspotProfileID); err == nil {
		t.Fatal("package service kind mismatch was accepted")
	}

	posResult, err := db.ExecContext(ctx, `INSERT INTO points_of_sale(code,name)
		VALUES('SHOP-001','Test grocery')`)
	if err != nil {
		t.Fatal(err)
	}
	posID, _ := posResult.LastInsertId()
	otherPosResult, err := db.ExecContext(ctx, `INSERT INTO points_of_sale(code,name)
		VALUES('SHOP-002','Other grocery')`)
	if err != nil {
		t.Fatal(err)
	}
	otherPosID, _ := otherPosResult.LastInsertId()

	if _, err := db.ExecContext(ctx, `INSERT INTO design_assets
		(asset_key,original_name,mime_type,size_bytes,sha256,width_px,height_px)
		VALUES('test-bg','background.png','image/png',128,'test-hash',1200,450)`); err != nil {
		t.Fatal(err)
	}
	assetID := int64(1)

	globalTemplateResult, err := db.ExecContext(ctx, `INSERT INTO card_templates
		(package_id,scope_kind,name,is_default) VALUES(?,'global','Default template',1)`, packageID)
	if err != nil {
		t.Fatal(err)
	}
	globalTemplateID, _ := globalTemplateResult.LastInsertId()
	globalVersionResult, err := db.ExecContext(ctx, `INSERT INTO card_template_versions
		(template_id,version_number,card_width_mm,card_height_mm,design_json,page_layout_json,background_asset_id)
		VALUES(?,1,50,19.1,'{"elements":[]}','{"paper":"A4","columns":4,"rows":15}',?)`, globalTemplateID, assetID)
	if err != nil {
		t.Fatal(err)
	}

	posTemplateResult, err := db.ExecContext(ctx, `INSERT INTO card_templates
		(package_id,point_of_sale_id,scope_kind,name,is_default)
		VALUES(?,?,'point_of_sale','Shop-specific template',0)`, packageID, posID)
	if err != nil {
		t.Fatal(err)
	}
	posTemplateID, _ := posTemplateResult.LastInsertId()
	posVersionResult, err := db.ExecContext(ctx, `INSERT INTO card_template_versions
		(template_id,version_number,card_width_mm,card_height_mm,design_json,page_layout_json)
		VALUES(?,1,50,19.1,'{"elements":[{"type":"username","x":10,"y":7}]}','{"paper":"A4","columns":4,"rows":15}')`, posTemplateID)
	if err != nil {
		t.Fatal(err)
	}
	posVersionID, _ := posVersionResult.LastInsertId()
	if _, err := db.ExecContext(ctx, `INSERT INTO point_of_sale_packages
		(point_of_sale_id,package_id,prefix_override,template_id_override)
		VALUES(?,?,?,?)`, posID, packageID, "SHOP-", posTemplateID); err != nil {
		t.Fatalf("valid package/POS template override failed: %v", err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO point_of_sale_packages
		(point_of_sale_id,package_id,template_id_override)
		VALUES(?,?,?)`, otherPosID, packageID, posTemplateID); err == nil {
		t.Fatal("template for one point of sale was accepted by another point of sale")
	}
	if _, err := db.ExecContext(ctx, `UPDATE card_template_versions SET design_json='{"elements":[]}' WHERE id=?`, posVersionID); err == nil {
		t.Fatal("immutable template version was modified in place")
	}

	batchResult, err := db.ExecContext(ctx, `INSERT INTO voucher_batches
		(reference,prefix,profile_name,server_name,limit_uptime,requested_count,status,
		router_id,package_id,point_of_sale_id,template_version_id,service_system_snapshot,
		package_name_snapshot,user_price_minor_snapshot,dealer_price_minor_snapshot)
		VALUES('VCH-DOMAIN-001','SHOP-','UM-200','all','',1,'completed',?,?,?,?,
		'usermanager','Abu 200',20000,16000)`, routerID, packageID, posID, posVersionID)
	if err != nil {
		t.Fatal(err)
	}
	batchID, _ := batchResult.LastInsertId()
	if _, err := db.ExecContext(ctx, `INSERT INTO vouchers
		(batch_id,username,profile_name,server_name,limit_uptime,package_id,point_of_sale_id)
		VALUES(?,'SHOP-0001','UM-200','all','',?,?)`, batchID, packageID, posID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO voucher_stock_movements
		(reference,movement_type,batch_id,package_id,to_point_of_sale_id,quantity)
		VALUES('STOCK-001','allocate',?,?,?,1)`, batchID, packageID, posID); err != nil {
		t.Fatal(err)
	}

	var mode string
	if err := db.QueryRowContext(ctx, `SELECT package_selection_mode FROM points_of_sale WHERE id=?`, posID).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "all" {
		t.Fatalf("new point of sale default selection mode=%q, want all", mode)
	}
	var snapshotName string
	var dealerPrice int64
	if err := db.QueryRowContext(ctx, `SELECT package_name_snapshot,dealer_price_minor_snapshot
		FROM voucher_batches WHERE id=?`, batchID).Scan(&snapshotName, &dealerPrice); err != nil {
		t.Fatal(err)
	}
	if snapshotName != "Abu 200" || dealerPrice != 16000 {
		t.Fatalf("batch commercial snapshot was not retained: name=%q dealer price=%d", snapshotName, dealerPrice)
	}

	var oldTableCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('voucher_products','pos_sales','pos_sale_items')`).Scan(&oldTableCount); err != nil {
		t.Fatal(err)
	}
	if oldTableCount != 3 {
		t.Fatalf("legacy POS tables missing after additive migration: found %d of 3", oldTableCount)
	}
	_ = globalVersionResult
}

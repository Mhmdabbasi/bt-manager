package store

import (
	"context"
	"errors"
	"testing"
)

func TestResolveCardTemplateVersionRequiresSavedVersionAndPrefersExplicitOverride(t *testing.T) {
	db, packageID, _ := setupPointOfSaleTestDB(t)
	ctx := context.Background()

	versionID, found, err := db.ResolveCardTemplateVersion(ctx, packageID, 0, 0)
	if err != nil || found || versionID != 0 {
		t.Fatalf("no-template resolution: version=%d found=%v err=%v", versionID, found, err)
	}

	result, err := db.ExecContext(ctx, `INSERT INTO card_templates(package_id,scope_kind,name,is_default,active)
		VALUES(?,'global','Global default',1,1)`, packageID)
	if err != nil {
		t.Fatal(err)
	}
	globalTemplateID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	result, err = db.ExecContext(ctx, `INSERT INTO card_template_versions
		(template_id,version_number,card_width_mm,card_height_mm,design_json,page_layout_json)
		VALUES(?,1,50,19.1,'{}','{}')`, globalTemplateID)
	if err != nil {
		t.Fatal(err)
	}
	globalVersionID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	versionID, found, err = db.ResolveCardTemplateVersion(ctx, packageID, 0, 0)
	if err != nil || !found || versionID != globalVersionID {
		t.Fatalf("global template resolution: version=%d found=%v err=%v", versionID, found, err)
	}

	pos, err := db.CreatePointOfSale(ctx, PointOfSaleInput{Code: "TEMPLATE-POS", Name: "Template POS", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	result, err = db.ExecContext(ctx, `INSERT INTO card_templates(package_id,point_of_sale_id,scope_kind,name,is_default,active)
		VALUES(?,?,'point_of_sale','POS override',0,1)`, packageID, pos.ID)
	if err != nil {
		t.Fatal(err)
	}
	overrideTemplateID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	result, err = db.ExecContext(ctx, `INSERT INTO card_template_versions
		(template_id,version_number,card_width_mm,card_height_mm,design_json,page_layout_json)
		VALUES(?,1,50,19.1,'{}','{}')`, overrideTemplateID)
	if err != nil {
		t.Fatal(err)
	}
	overrideVersionID, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}

	versionID, found, err = db.ResolveCardTemplateVersion(ctx, packageID, pos.ID, overrideTemplateID)
	if err != nil || !found || versionID != overrideVersionID {
		t.Fatalf("POS override resolution: version=%d want=%d found=%v err=%v", versionID, overrideVersionID, found, err)
	}
	if _, _, err := db.ResolveCardTemplateVersion(ctx, packageID, pos.ID, 9999); !errors.Is(err, ErrCardTemplateVersionNotFound) {
		t.Fatalf("missing override error=%v", err)
	}
}

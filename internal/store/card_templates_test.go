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

func TestSaveCardTemplateVersionCreatesImmutableVersionsAndDefault(t *testing.T) {
	db, packageID, _ := setupPointOfSaleTestDB(t)
	ctx := context.Background()
	first, err := db.SaveCardTemplateVersion(ctx, SaveCardTemplateInput{
		PackageID: packageID, Name: "Voucher template", IsDefault: true,
		CardWidthMM: 50, CardHeightMM: 19.1,
		DesignJSON: []byte(`[{"id":"username","field":"username","x":4,"y":40}]`),
		PageLayoutJSON: []byte(`{"columns":4,"rows":15,"marginMm":5,"gapMm":1}`),
	})
	if err != nil { t.Fatal(err) }
	if first.ID < 1 || first.VersionNumber != 1 || !first.IsDefault || first.LatestVersionID == nil {
		t.Fatalf("unexpected first version: %+v", first)
	}
	second, err := db.SaveCardTemplateVersion(ctx, SaveCardTemplateInput{
		ID: first.ID, PackageID: packageID, Name: "Voucher template", IsDefault: true,
		CardWidthMM: 50, CardHeightMM: 19.1,
		DesignJSON: []byte(`[{"id":"username","field":"username","x":8,"y":42}]`),
		PageLayoutJSON: []byte(`{"columns":4,"rows":15,"marginMm":5,"gapMm":1}`),
	})
	if err != nil { t.Fatal(err) }
	if second.ID != first.ID || second.VersionNumber != 2 || second.LatestVersionID == nil || *second.LatestVersionID == *first.LatestVersionID {
		t.Fatalf("version was not incremented: first=%+v second=%+v", first, second)
	}
	items, err := db.ListCardTemplates(ctx, packageID)
	if err != nil { t.Fatal(err) }
	if len(items) != 1 || items[0].VersionNumber != 2 || !items[0].IsDefault {
		t.Fatalf("unexpected templates: %+v", items)
	}
	if _, err := db.SaveCardTemplateVersion(ctx, SaveCardTemplateInput{PackageID:packageID,Name:"Invalid",CardWidthMM:0,CardHeightMM:19.1}); err == nil {
		t.Fatal("expected invalid dimensions to be rejected")
	}
	pos,err:=db.CreatePointOfSale(ctx,PointOfSaleInput{Code:"TEMPLATE-SCOPE",Name:"Template Scope POS",Active:true})
	if err!=nil { t.Fatal(err) }
	posTemplate,err:=db.SaveCardTemplateVersion(ctx,SaveCardTemplateInput{PackageID:packageID,PointOfSaleID:pos.ID,Name:"POS card",IsDefault:true,CardWidthMM:50,CardHeightMM:19.1,DesignJSON:[]byte(`[]`),PageLayoutJSON:[]byte(`{}`)})
	if err!=nil { t.Fatal(err) }
	if posTemplate.ScopeKind!="point_of_sale"||posTemplate.IsDefault { t.Fatalf("unexpected POS template: %+v",posTemplate) }
	versionID,found,err:=db.ResolveCardTemplateVersion(ctx,packageID,pos.ID,posTemplate.ID)
	if err!=nil||!found||posTemplate.LatestVersionID==nil||versionID!=*posTemplate.LatestVersionID { t.Fatalf("POS template override: version=%d found=%v err=%v template=%+v",versionID,found,err,posTemplate) }
	var override int64
	if err:=db.QueryRowContext(ctx,`SELECT template_id_override FROM point_of_sale_packages WHERE point_of_sale_id=? AND package_id=?`,pos.ID,packageID).Scan(&override);err!=nil||override!=posTemplate.ID { t.Fatalf("POS package override=%d err=%v",override,err) }
}

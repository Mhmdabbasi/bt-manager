package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func setupPointOfSaleTestDB(t *testing.T) (*DB, int64, int64) {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "points-of-sale.db"))
	if err != nil { t.Fatal(err) }
	t.Cleanup(func(){ _ = db.Close() })
	ctx := context.Background()
	synced, err := db.SyncHotSpotProfilesSnapshot(ctx, HotSpotProfileSyncInput{
		Address:"192.0.2.31",Port:8728,RouterIdentity:"pos-router",RouterOSVersion:"7.20.1",
		Profiles:[]ImportedServiceProfile{{SourceID:"*1",Name:"basic"},{SourceID:"*2",Name:"premium"}},
	})
	if err != nil { t.Fatal(err) }
	pkg1, err := db.CreateCardPackage(ctx,CardPackageInput{
		Name:"Basic 100",ServiceSystem:"hotspot",ServiceProfileID:1,
		UserPriceMinor:10000,DealerPriceMinor:8000,DefaultPrefix:"B100-",Active:true,
	})
	if err != nil { t.Fatal(err) }
	pkg2, err := db.CreateCardPackage(ctx,CardPackageInput{
		Name:"Premium 200",ServiceSystem:"hotspot",ServiceProfileID:2,
		UserPriceMinor:20000,DealerPriceMinor:15000,DefaultPrefix:"P200-",Active:true,
	})
	if err != nil { t.Fatal(err) }
	if synced.RouterID<1 { t.Fatal("router was not imported") }
	return db,pkg1.ID,pkg2.ID
}

func TestPointOfSaleCRUDAndArchive(t *testing.T) {
	db,_,_:=setupPointOfSaleTestDB(t)
	ctx:=context.Background()
	created,err:=db.CreatePointOfSale(ctx,PointOfSaleInput{
		Code:" shop-01 ",Name:"Kiosk One",Phone:"777123456",Address:"Main Street",
		PackageSelectionMode:"all",Active:true,
	})
	if err!=nil { t.Fatal(err) }
	if created.Code!="SHOP-01"||created.Name!="Kiosk One"||!created.Active||created.PackageSelectionMode!="all" {
		t.Fatalf("unexpected POS: %+v",created)
	}
	updated,err:=db.UpdatePointOfSale(ctx,created.ID,PointOfSaleInput{
		Code:"KIOSK-01",Name:"Kiosk Updated",Phone:"777123457",Address:"Second Street",
		Notes:"updated",PackageSelectionMode:"selected",Active:true,
	})
	if err!=nil { t.Fatal(err) }
	if updated.Code!="KIOSK-01"||updated.PackageSelectionMode!="selected"||updated.Notes!="updated" {
		t.Fatalf("unexpected updated POS: %+v",updated)
	}
	archived,err:=db.ArchivePointOfSale(ctx,created.ID)
	if err!=nil { t.Fatal(err) }
	if archived.Active { t.Fatalf("POS remains active after archive: %+v",archived) }
	active,err:=db.ListPointsOfSale(ctx,false)
	if err!=nil||len(active)!=0 { t.Fatalf("active list=%+v err=%v",active,err) }
	all,err:=db.ListPointsOfSale(ctx,true)
	if err!=nil||len(all)!=1||all[0].ID!=created.ID||all[0].Active {
		t.Fatalf("archive removed POS history: %+v err=%v",all,err)
	}
}

func TestPointOfSaleCodeIsNormalizedAndUnique(t *testing.T) {
	db,_,_:=setupPointOfSaleTestDB(t)
	ctx:=context.Background()
	_,err:=db.CreatePointOfSale(ctx,PointOfSaleInput{Code:"shop-a",Name:"Shop A",Active:true})
	if err!=nil { t.Fatal(err) }
	_,err=db.CreatePointOfSale(ctx,PointOfSaleInput{Code:"SHOP-A",Name:"Shop B",Active:true})
	if !errors.Is(err,ErrPointOfSaleCodeExists) { t.Fatalf("duplicate code error=%v",err) }
	_,err=db.CreatePointOfSale(ctx,PointOfSaleInput{Code:"BAD CODE",Name:"Bad code",Active:true})
	if !errors.Is(err,ErrInvalidPointOfSale) { t.Fatalf("invalid code error=%v",err) }
}

func TestPointOfSalePackageRulesRespectAllAndSelectedModes(t *testing.T) {
	db,pkg1,pkg2:=setupPointOfSaleTestDB(t)
	ctx:=context.Background()
	pos,err:=db.CreatePointOfSale(ctx,PointOfSaleInput{Code:"SHOP-02",Name:"Shop Two",PackageSelectionMode:"all",Active:true})
	if err!=nil { t.Fatal(err) }

	all,err:=db.ListPointOfSalePackageRules(ctx,pos.ID,true)
	if err!=nil { t.Fatal(err) }
	if len(all)!=2||!all[0].IsEnabled||!all[1].IsEnabled||all[0].HasRule||all[1].HasRule {
		t.Fatalf("all-mode default should allow packages with no rules: %+v",all)
	}
	prefix:="VIP-"
	selected,err:=db.ReplacePointOfSalePackageRules(ctx,pos.ID,"selected",[]PointOfSalePackageRuleInput{
		{PackageID:pkg1,IsEnabled:true,PrefixOverride:&prefix},
		{PackageID:pkg2,IsEnabled:false},
	})
	if err!=nil { t.Fatal(err) }
	if len(selected)!=2 { t.Fatalf("selected mode result=%+v",selected) }
	for _,rule:=range selected {
		if rule.PackageID==pkg1 {
			if !rule.IsEnabled||!rule.HasRule||rule.PrefixOverride==nil||*rule.PrefixOverride!="VIP-" {
				t.Fatalf("selected package rule wasn't saved: %+v",rule)
			}
		}
		if rule.PackageID==pkg2&&rule.IsEnabled { t.Fatalf("unselected package is enabled: %+v",rule) }
	}
	// An empty submitted ruleset in all mode means every package is allowed by default.
	all,err=db.ReplacePointOfSalePackageRules(ctx,pos.ID,"all",[]PointOfSalePackageRuleInput{
		{PackageID:pkg1,IsEnabled:true,PrefixOverride:&prefix},
		{PackageID:pkg2,IsEnabled:true},
	})
	if err!=nil { t.Fatal(err) }
	for _,rule:=range all {
		if !rule.IsEnabled { t.Fatalf("all-mode package is unexpectedly disabled: %+v",rule) }
	}
	pos,err=db.GetPointOfSale(ctx,pos.ID)
	if err!=nil||pos.PackageSelectionMode!="all" { t.Fatalf("POS mode not persisted: %+v err=%v",pos,err) }
}

func TestPointOfSalePackageRulesRejectInvalidIDsAndPrefixesAtomically(t *testing.T) {
	db,pkg1,_:=setupPointOfSaleTestDB(t)
	ctx:=context.Background()
	pos,err:=db.CreatePointOfSale(ctx,PointOfSaleInput{Code:"SHOP-03",Name:"Shop Three",PackageSelectionMode:"all",Active:true})
	if err!=nil { t.Fatal(err) }
	badPrefix:="اسم عربي"
	if _,err=db.ReplacePointOfSalePackageRules(ctx,pos.ID,"selected",[]PointOfSalePackageRuleInput{
		{PackageID:pkg1,IsEnabled:true,PrefixOverride:&badPrefix},
	});!errors.Is(err,ErrInvalidPointOfSalePackageRules) {
		t.Fatalf("invalid prefix error=%v",err)
	}
	if _,err=db.ReplacePointOfSalePackageRules(ctx,pos.ID,"selected",[]PointOfSalePackageRuleInput{
		{PackageID:pkg1,IsEnabled:true},{PackageID:pkg1,IsEnabled:false},
	});!errors.Is(err,ErrInvalidPointOfSalePackageRules) {
		t.Fatalf("duplicate package rule error=%v",err)
	}
	if _,err=db.ReplacePointOfSalePackageRules(ctx,pos.ID,"all",[]PointOfSalePackageRuleInput{
		{PackageID:999,IsEnabled:true},
	});!errors.Is(err,ErrPointOfSalePackageNotFound) {
		t.Fatalf("unknown package error=%v",err)
	}
	unchanged,err:=db.GetPointOfSale(ctx,pos.ID)
	if err!=nil||unchanged.PackageSelectionMode!="all" { t.Fatalf("failed transaction changed POS mode: %+v err=%v",unchanged,err) }
	rules,err:=db.ListPointOfSalePackageRules(ctx,pos.ID,true)
	if err!=nil||len(rules)!=2||!rules[0].IsEnabled||!rules[1].IsEnabled {
		t.Fatalf("failed transaction changed rules: %+v err=%v",rules,err)
	}
}

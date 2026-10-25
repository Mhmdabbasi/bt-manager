package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestVoucherBatchStoresOnlySuccessfulVoucherEntries(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	batch, err := db.BeginVoucherBatch(ctx, NewVoucherBatch{
		Reference: "VCH-TEST-001", Prefix: "BT-", ProfileName: "1M",
		ServerName: "all", RequestedCount: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddVoucher(ctx, batch.ID, "BT-AAAA2222"); err != nil {
		t.Fatal(err)
	}
	finished, err := db.FinishVoucherBatch(ctx, batch.ID, 1, "router rejected another account")
	if err != nil {
		t.Fatal(err)
	}
	if finished.Status != "partial" || finished.CreatedCount != 1 || finished.FailedCount != 1 {
		t.Fatalf("unexpected finished batch: %+v", finished)
	}
	vouchers, err := db.ListVouchersByBatch(ctx, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(vouchers) != 1 || vouchers[0].Username != "BT-AAAA2222" {
		t.Fatalf("unexpected vouchers: %+v", vouchers)
	}
}

func TestVoucherUsernameIsUniqueAcrossBatches(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	for _, ref := range []string{"VCH-TEST-101", "VCH-TEST-102"} {
		batch, err := db.BeginVoucherBatch(ctx, NewVoucherBatch{
			Reference: ref, ProfileName: "default", RequestedCount: 1,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.AddVoucher(ctx, batch.ID, "same-user"); err == nil && ref == "VCH-TEST-102" {
			t.Fatal("duplicate voucher username was accepted")
		}
		if ref == "VCH-TEST-101" {
			if _, err := db.FinishVoucherBatch(ctx, batch.ID, 0, ""); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestVoucherBatchPersistsCredentialOptionsAndPasswords(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "credentials.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	batch, err := db.BeginVoucherBatch(ctx, NewVoucherBatch{
		Reference: "VCH-TEST-CREDENTIALS", Prefix: "NET-", ProfileName: "default",
		RequestedCount: 1, UsernameLength: 10, UsernameMode: "numbers",
		PasswordMode: "random", PasswordLength: 6, PasswordCharset: "letters",
	})
	if err != nil {
		t.Fatal(err)
	}
	if batch.UsernameLength != 10 || batch.UsernameMode != "numbers" || batch.PasswordMode != "random" || batch.PasswordLength != 6 || batch.PasswordCharset != "letters" {
		t.Fatalf("credential options were not persisted: %+v", batch)
	}
	if _, err := db.AddVoucher(ctx, batch.ID, "NET-1234567890", "PASSXY"); err != nil {
		t.Fatal(err)
	}
	vouchers, err := db.ListVouchersByBatch(ctx, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(vouchers) != 1 || vouchers[0].Password != "PASSXY" {
		t.Fatalf("voucher password was not persisted: %+v", vouchers)
	}
}

func TestPackageVoucherBatchSnapshotsAndStockLedger(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "package-voucher-batch.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()

	synced, err := db.SyncHotSpotProfilesSnapshot(ctx, HotSpotProfileSyncInput{
		Address: "192.0.2.55", Port: 8728, RouterIdentity: "snapshot-router", RouterOSVersion: "7.20.1",
		Profiles: []ImportedServiceProfile{{SourceID: "*1", Name: "premium", Properties: map[string]string{"rate-limit": "8M/8M", "shared-users": "1"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := db.ListServiceProfiles(ctx, synced.RouterID, "hotspot", true)
	if err != nil || len(profiles) != 1 {
		t.Fatalf("profiles=%+v err=%v", profiles, err)
	}
	pkg, err := db.CreateCardPackage(ctx, CardPackageInput{
		Name: "Premium 200", ServiceSystem: "hotspot", ServiceProfileID: profiles[0].ID,
		UserPriceMinor: 20000, DealerPriceMinor: 15000, DefaultPrefix: "P200-", Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	pos, err := db.CreatePointOfSale(ctx, PointOfSaleInput{
		Code: "SHOP-55", Name: "Snapshot Shop", PackageSelectionMode: "all", Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	userPrice, dealerPrice := pkg.UserPriceMinor, pkg.DealerPriceMinor
	batch, err := db.BeginVoucherBatch(ctx, NewVoucherBatch{
		Reference: "VCH-PACKAGE-SNAPSHOT", Prefix: "P200-", ProfileName: pkg.ServiceProfileName,
		ServerName: "all", RequestedCount: 2, RouterID: pkg.RouterID, PackageID: pkg.ID,
		PointOfSaleID: pos.ID, ServiceSystemSnapshot: pkg.ServiceSystem, PackageNameSnapshot: pkg.Name,
		UserPriceMinorSnapshot: &userPrice, DealerPriceMinorSnapshot: &dealerPrice,
		GenerationConfigJSON: `{"count":2,"usernameMode":"numbers"}`,
		ServiceProfileSnapshotJSON: `{"rate-limit":"8M/8M","shared-users":"1"}`,
		LimitationSnapshotJSON: "[]",
	})
	if err != nil {
		t.Fatal(err)
	}
	if batch.PackageID == nil || *batch.PackageID != pkg.ID || batch.PointOfSaleID == nil || *batch.PointOfSaleID != pos.ID ||
		batch.PackageNameSnapshot != pkg.Name || batch.UserPriceMinorSnapshot == nil || *batch.UserPriceMinorSnapshot != 20000 ||
		batch.GenerationConfig["usernameMode"] != "numbers" || batch.ServiceProfileSnapshot["rate-limit"] != "8M/8M" {
		t.Fatalf("package snapshots were not returned: %+v", batch)
	}
	for _, username := range []string{"P200-1111", "P200-2222"} {
		if _, err := db.AddVoucher(ctx, batch.ID, username); err != nil {
			t.Fatal(err)
		}
	}
	var storedPackageID, storedPointID sql.NullInt64
	if err := db.QueryRowContext(ctx, "SELECT package_id,point_of_sale_id FROM vouchers WHERE batch_id=? ORDER BY id LIMIT 1", batch.ID).
		Scan(&storedPackageID, &storedPointID); err != nil {
		t.Fatal(err)
	}
	if !storedPackageID.Valid || storedPackageID.Int64 != pkg.ID || !storedPointID.Valid || storedPointID.Int64 != pos.ID {
		t.Fatalf("voucher package assignment was not copied from batch: package=%+v point=%+v", storedPackageID, storedPointID)
	}
	finished, err := db.FinishVoucherBatch(ctx, batch.ID, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if finished.Status != "completed" || finished.CreatedCount != 2 || finished.PackageNameSnapshot != pkg.Name {
		t.Fatalf("unexpected finished packaged batch: %+v", finished)
	}
	var issueCount, allocationCount, issueQuantity, allocationQuantity int
	if err := db.QueryRowContext(ctx, "SELECT count(*),COALESCE(sum(quantity),0) FROM voucher_stock_movements WHERE batch_id=? AND movement_type='issue'", batch.ID).
		Scan(&issueCount, &issueQuantity); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*),COALESCE(sum(quantity),0) FROM voucher_stock_movements WHERE batch_id=? AND movement_type='allocate' AND to_point_of_sale_id=?", batch.ID, pos.ID).
		Scan(&allocationCount, &allocationQuantity); err != nil {
		t.Fatal(err)
	}
	if issueCount != 1 || issueQuantity != 2 || allocationCount != 1 || allocationQuantity != 2 {
		t.Fatalf("unexpected stock movements: issue=(%d,%d) allocation=(%d,%d)", issueCount, issueQuantity, allocationCount, allocationQuantity)
	}
	if _, err := db.FinishVoucherBatch(ctx, batch.ID, 0, "duplicate finalize"); err != nil {
		t.Fatalf("finalizing an already completed batch should be idempotent: %v", err)
	}
	var movementCount int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM voucher_stock_movements WHERE batch_id=?", batch.ID).Scan(&movementCount); err != nil {
		t.Fatal(err)
	}
	if movementCount != 2 {
		t.Fatalf("repeat finalize duplicated inventory movements: %d", movementCount)
	}
}

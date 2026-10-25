package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func openPOSStore(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "pos.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func seedVouchers(t *testing.T, db *DB, names ...string) {
	t.Helper()
	ctx := context.Background()
	batch, err := db.BeginVoucherBatch(ctx, NewVoucherBatch{
		Reference: "VCH-SEED-" + names[0], ProfileName: "default",
		ServerName: "all", RequestedCount: len(names),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if _, err := db.AddVoucher(ctx, batch.ID, name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.FinishVoucherBatch(ctx, batch.ID, 0, ""); err != nil {
		t.Fatal(err)
	}
}

func TestPOSSaleAtomicallySellsVouchersAndLocksCurrency(t *testing.T) {
	db := openPOSStore(t)
	ctx := context.Background()
	seedVouchers(t, db, "VOUCHER-A", "VOUCHER-B", "VOUCHER-C")
	product, err := db.CreateVoucherProduct(ctx, VoucherProductInput{
		Name: "باقة 200", ProfileName: "default", PriceMinor: 20000, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	sale, err := db.CreatePOSSale(ctx, "POS-TEST-0001", product.ID, 2, "cash")
	if err != nil {
		t.Fatal(err)
	}
	if sale.Sale.TotalMinor != 40000 || len(sale.Items) != 2 {
		t.Fatalf("unexpected receipt: %+v", sale)
	}
	if sale.Items[0].Username == "" || sale.Items[1].Username == "" {
		t.Fatalf("sale receipt must contain the issued voucher usernames: %+v", sale.Items)
	}
	products, err := db.ListVoucherProducts(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 1 || products[0].StockCount != 1 {
		t.Fatalf("unexpected remaining stock: %+v", products)
	}
	var transactions int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM financial_transactions WHERE reference=? AND amount_minor=?`, sale.Sale.Reference, int64(40000)).Scan(&transactions); err != nil {
		t.Fatal(err)
	}
	if transactions != 1 {
		t.Fatalf("financial transaction count = %d, want 1", transactions)
	}
	var saleMovementCount, saleMovementQuantity int
	if err := db.QueryRowContext(ctx, `SELECT count(*),COALESCE(sum(quantity),0)
		FROM voucher_stock_movements WHERE movement_type='sale' AND reference LIKE ?`, sale.Sale.Reference+"-SALE-%").
		Scan(&saleMovementCount, &saleMovementQuantity); err != nil {
		t.Fatal(err)
	}
	if saleMovementCount != 1 || saleMovementQuantity != 2 {
		t.Fatalf("sale stock ledger = (%d movements, quantity %d), want (1, 2)", saleMovementCount, saleMovementQuantity)
	}

	settings, err := db.GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.CurrencyCode, settings.CurrencySymbol = "SAR", "ر.س"
	if err := db.SaveSettings(ctx, settings); !errors.Is(err, ErrCurrencyLocked) {
		t.Fatalf("expected currency lock after a sale, got %v", err)
	}
}

func TestPOSSaleRejectsInsufficientStockWithoutPartialSale(t *testing.T) {
	db := openPOSStore(t)
	ctx := context.Background()
	seedVouchers(t, db, "VOUCHER-X")
	product, err := db.CreateVoucherProduct(ctx, VoucherProductInput{
		Name: "Limited", ProfileName: "default", PriceMinor: 500, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreatePOSSale(ctx, "POS-TEST-0002", product.ID, 2, "cash"); !errors.Is(err, ErrInsufficientVoucherStock) {
		t.Fatalf("sale error = %v, want ErrInsufficientVoucherStock", err)
	}
	var sales, transactions int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM pos_sales`).Scan(&sales); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM financial_transactions`).Scan(&transactions); err != nil {
		t.Fatal(err)
	}
	if sales != 0 || transactions != 0 {
		t.Fatalf("failed sale was partially recorded: sales=%d transactions=%d", sales, transactions)
	}
}

func TestPOSSaleSameReferenceIsIdempotent(t *testing.T) {
	db := openPOSStore(t)
	ctx := context.Background()
	seedVouchers(t, db, "VOUCHER-R1", "VOUCHER-R2")
	product, err := db.CreateVoucherProduct(ctx, VoucherProductInput{
		Name: "Retry-safe", ProfileName: "default", PriceMinor: 1000, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := db.CreatePOSSale(ctx, "POS-TEST-0003", product.ID, 1, "cash")
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.CreatePOSSale(ctx, "POS-TEST-0003", product.ID, 1, "cash")
	if err != nil {
		t.Fatal(err)
	}
	if first.Sale.ID != second.Sale.ID || first.Items[0].Username != second.Items[0].Username {
		t.Fatalf("retry returned a different receipt: first=%+v second=%+v", first, second)
	}
	var sales, transactions int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM pos_sales`).Scan(&sales); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM financial_transactions`).Scan(&transactions); err != nil {
		t.Fatal(err)
	}
	if sales != 1 || transactions != 1 {
		t.Fatalf("retry duplicated the sale: sales=%d transactions=%d", sales, transactions)
	}
}

func TestPOSProductStockIsScopedToHotSpotServer(t *testing.T) {
	db := openPOSStore(t)
	ctx := context.Background()

	serverBatch, err := db.BeginVoucherBatch(ctx, NewVoucherBatch{
		Reference: "VCH-SERVER-101", ProfileName: "default",
		ServerName: "hotspot1", LimitUptime: "1d00:00:00", RequestedCount: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddVoucher(ctx, serverBatch.ID, "USER-HOTSPOT1-TIMED"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.FinishVoucherBatch(ctx, serverBatch.ID, 0, ""); err != nil {
		t.Fatal(err)
	}

	noLimitBatch, err := db.BeginVoucherBatch(ctx, NewVoucherBatch{
		Reference: "VCH-SERVER-103", ProfileName: "default",
		ServerName: "hotspot1", RequestedCount: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddVoucher(ctx, noLimitBatch.ID, "USER-HOTSPOT1-NOLIMIT"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.FinishVoucherBatch(ctx, noLimitBatch.ID, 0, ""); err != nil {
		t.Fatal(err)
	}

	allBatch, err := db.BeginVoucherBatch(ctx, NewVoucherBatch{
		Reference: "VCH-SERVER-102", ProfileName: "default",
		ServerName: "all", RequestedCount: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddVoucher(ctx, allBatch.ID, "USER-ALL"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.FinishVoucherBatch(ctx, allBatch.ID, 0, ""); err != nil {
		t.Fatal(err)
	}

	product, err := db.CreateVoucherProduct(ctx, VoucherProductInput{
		Name: "Hotspot 1 timed pack", ProfileName: "default", ServerName: "hotspot1",
		LimitUptime: "1d00:00:00", PriceMinor: 15000, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if product.StockCount != 1 {
		t.Fatalf("server-specific stock = %d, want 1", product.StockCount)
	}
	sale, err := db.CreatePOSSale(ctx, "POS-TEST-SERVER-01", product.ID, 1, "cash")
	if err != nil {
		t.Fatal(err)
	}
	if len(sale.Items) != 1 || sale.Items[0].Username != "USER-HOTSPOT1-TIMED" || sale.Items[0].ServerName != "hotspot1" || sale.Items[0].LimitUptime != "1d00:00:00" {
		t.Fatalf("POS sold a voucher from the wrong HotSpot server: %+v", sale.Items)
	}
}


func TestLegacyPOSCannotSellPackageBackedVouchers(t *testing.T) {
	db := openPOSStore(t)
	ctx := context.Background()

	synced, err := db.SyncHotSpotProfilesSnapshot(ctx, HotSpotProfileSyncInput{
		Address: "192.0.2.77", Port: 8728, RouterIdentity: "package-stock-router", RouterOSVersion: "7.20.1",
		Profiles: []ImportedServiceProfile{{SourceID: "*1", Name: "shared-profile", Properties: map[string]string{"rate-limit": "4M/4M"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := db.ListServiceProfiles(ctx, synced.RouterID, "hotspot", true)
	if err != nil || len(profiles) != 1 {
		t.Fatalf("profiles=%+v err=%v", profiles, err)
	}
	pkg, err := db.CreateCardPackage(ctx, CardPackageInput{
		Name: "Package-backed 200", ServiceSystem: "hotspot", ServiceProfileID: profiles[0].ID,
		UserPriceMinor: 20000, DealerPriceMinor: 15000, DefaultPrefix: "PKG-", Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	userPrice, dealerPrice := pkg.UserPriceMinor, pkg.DealerPriceMinor
	batch, err := db.BeginVoucherBatch(ctx, NewVoucherBatch{
		Reference: "VCH-POS-PACKAGE-ISOLATION", Prefix: "PKG-", ProfileName: pkg.ServiceProfileName,
		ServerName: "all", RequestedCount: 1, RouterID: pkg.RouterID, PackageID: pkg.ID,
		ServiceSystemSnapshot: pkg.ServiceSystem, PackageNameSnapshot: pkg.Name,
		UserPriceMinorSnapshot: &userPrice, DealerPriceMinorSnapshot: &dealerPrice,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.AddVoucher(ctx, batch.ID, "PKG-VOUCHER-001"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.FinishVoucherBatch(ctx, batch.ID, 0, ""); err != nil {
		t.Fatal(err)
	}

	legacyProduct, err := db.CreateVoucherProduct(ctx, VoucherProductInput{
		Name: "Legacy same-profile product", ProfileName: pkg.ServiceProfileName,
		ServerName: "all", PriceMinor: 20000, Active: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if legacyProduct.StockCount != 0 {
		t.Fatalf("legacy POS counted package-backed stock: got %d, want 0", legacyProduct.StockCount)
	}
	if _, err := db.CreatePOSSale(ctx, "POS-PACKAGE-ISOLATION-01", legacyProduct.ID, 1, "cash"); !errors.Is(err, ErrInsufficientVoucherStock) {
		t.Fatalf("legacy POS sale error = %v, want ErrInsufficientVoucherStock", err)
	}
	var sales, movements int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM pos_sales").Scan(&sales); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM voucher_stock_movements WHERE movement_type='sale'").Scan(&movements); err != nil {
		t.Fatal(err)
	}
	if sales != 0 || movements != 0 {
		t.Fatalf("legacy sale changed package-backed stock: sales=%d sale movements=%d", sales, movements)
	}
}

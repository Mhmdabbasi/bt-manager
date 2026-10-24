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

package store

import (
	"context"
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

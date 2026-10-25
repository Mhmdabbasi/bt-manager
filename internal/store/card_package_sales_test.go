package store

import (
	"context"
	"errors"
	"testing"
)

func TestCardPackageSaleConsumesOnlyAssignedPackageStockAtomically(t *testing.T) {
	db, packageID, _ := setupPointOfSaleTestDB(t)
	ctx := context.Background()
	pos, err := db.CreatePointOfSale(ctx, PointOfSaleInput{Code:"SALE-POS-01", Name:"Sale kiosk", PackageSelectionMode:"all", Active:true})
	if err != nil { t.Fatal(err) }
	pkg, err := db.GetCardPackage(ctx, packageID)
	if err != nil { t.Fatal(err) }
	userPrice, dealerPrice := pkg.UserPriceMinor, pkg.DealerPriceMinor
	batch, err := db.BeginVoucherBatch(ctx, NewVoucherBatch{
		Reference:"VCH-PACKAGE-POS-SALE-01", Prefix:pkg.DefaultPrefix, ProfileName:pkg.ServiceProfileName,
		ServerName:"all", RequestedCount:2, RouterID:pkg.RouterID, PackageID:pkg.ID, PointOfSaleID:pos.ID,
		ServiceSystemSnapshot:pkg.ServiceSystem, PackageNameSnapshot:pkg.Name,
		UserPriceMinorSnapshot:&userPrice, DealerPriceMinorSnapshot:&dealerPrice,
	})
	if err != nil { t.Fatal(err) }
	for _, username := range []string{"PKG-POS-001","PKG-POS-002"} {
		if _, err := db.AddVoucher(ctx,batch.ID,username); err != nil { t.Fatal(err) }
	}
	if _, err := db.FinishVoucherBatch(ctx,batch.ID,0,""); err != nil { t.Fatal(err) }

	sale, err := db.CreateCardPackageSale(ctx,"CPOS-TEST-0001",pos.ID,packageID,1,"cash")
	if err != nil { t.Fatal(err) }
	if sale.Sale.TotalMinor != pkg.UserPriceMinor || len(sale.Items)!=1 || sale.Items[0].PackageID!=packageID {
		t.Fatalf("unexpected package sale receipt: %+v",sale)
	}
	if sale.Items[0].Username!="PKG-POS-001" {
		t.Fatalf("sale selected unexpected voucher: %+v",sale.Items[0])
	}
	var status string
	if err:=db.QueryRowContext(ctx,"SELECT status FROM vouchers WHERE id=?",sale.Items[0].VoucherID).Scan(&status);err!=nil{t.Fatal(err)}
	if status!="sold"{t.Fatalf("voucher status=%q, want sold",status)}
	var movements, movementQuantity, financials int
	if err:=db.QueryRowContext(ctx,`SELECT count(*),COALESCE(sum(quantity),0) FROM voucher_stock_movements
		WHERE movement_type='sale' AND reference LIKE 'CPOS-TEST-0001-SALE-%'`).Scan(&movements,&movementQuantity);err!=nil{t.Fatal(err)}
	if err:=db.QueryRowContext(ctx,"SELECT count(*) FROM financial_transactions WHERE reference='CPOS-TEST-0001' AND transaction_type='sale'").Scan(&financials);err!=nil{t.Fatal(err)}
	if movements!=1||movementQuantity!=1||financials!=1{t.Fatalf("ledger mismatch: movements=%d quantity=%d financials=%d",movements,movementQuantity,financials)}

	retry,err:=db.CreateCardPackageSale(ctx,"CPOS-TEST-0001",pos.ID,packageID,1,"cash")
	if err!=nil{t.Fatal(err)}
	if retry.Sale.ID!=sale.Sale.ID||len(retry.Items)!=1||retry.Items[0].VoucherID!=sale.Items[0].VoucherID{
		t.Fatalf("idempotent retry returned a different receipt: first=%+v retry=%+v",sale,retry)
	}
	var saleCount int
	if err:=db.QueryRowContext(ctx,"SELECT count(*) FROM card_package_sales WHERE reference='CPOS-TEST-0001'").Scan(&saleCount);err!=nil{t.Fatal(err)}
	if saleCount!=1{t.Fatalf("retry created %d sales, want 1",saleCount)}
}

func TestCardPackageSaleRejectsUnassignedAndDisallowedStock(t *testing.T) {
	db, packageID, _ := setupPointOfSaleTestDB(t)
	ctx:=context.Background()
	pos,err:=db.CreatePointOfSale(ctx,PointOfSaleInput{Code:"SALE-POS-02",Name:"Restricted kiosk",PackageSelectionMode:"selected",Active:true})
	if err!=nil{t.Fatal(err)}
	if _,err=db.CreateCardPackageSale(ctx,"CPOS-TEST-0002",pos.ID,packageID,1,"cash");!errors.Is(err,ErrCardPackageSaleNotAllowed){
		t.Fatalf("disallowed package error=%v, want ErrCardPackageSaleNotAllowed",err)
	}
	if _,err=db.CreateCardPackageSale(ctx,"CPOS-TEST-0003",pos.ID,999,1,"cash");!errors.Is(err,ErrCardPackageSaleNotAllowed){
		t.Fatalf("unknown package error=%v, want ErrCardPackageSaleNotAllowed",err)
	}
	var sales,transactions int
	if err:=db.QueryRowContext(ctx,"SELECT count(*) FROM card_package_sales").Scan(&sales);err!=nil{t.Fatal(err)}
	if err:=db.QueryRowContext(ctx,"SELECT count(*) FROM financial_transactions").Scan(&transactions);err!=nil{t.Fatal(err)}
	if sales!=0||transactions!=0{t.Fatalf("rejected sale changed records: sales=%d transactions=%d",sales,transactions)}
}

func TestCardPackageSaleCannotConsumeAnotherPOSOrPackageStock(t *testing.T) {
	db, packageID, otherPackageID := setupPointOfSaleTestDB(t)
	ctx:=context.Background()
	posA,err:=db.CreatePointOfSale(ctx,PointOfSaleInput{Code:"SALE-POS-03",Name:"Kiosk A",PackageSelectionMode:"all",Active:true})
	if err!=nil{t.Fatal(err)}
	posB,err:=db.CreatePointOfSale(ctx,PointOfSaleInput{Code:"SALE-POS-04",Name:"Kiosk B",PackageSelectionMode:"all",Active:true})
	if err!=nil{t.Fatal(err)}
	pkg,err:=db.GetCardPackage(ctx,packageID);if err!=nil{t.Fatal(err)}
	userPrice,dealerPrice:=pkg.UserPriceMinor,pkg.DealerPriceMinor
	batch,err:=db.BeginVoucherBatch(ctx,NewVoucherBatch{
		Reference:"VCH-PACKAGE-POS-SALE-02",Prefix:pkg.DefaultPrefix,ProfileName:pkg.ServiceProfileName,
		ServerName:"all",RequestedCount:1,RouterID:pkg.RouterID,PackageID:pkg.ID,PointOfSaleID:posA.ID,
		ServiceSystemSnapshot:pkg.ServiceSystem,PackageNameSnapshot:pkg.Name,
		UserPriceMinorSnapshot:&userPrice,DealerPriceMinorSnapshot:&dealerPrice,
	})
	if err!=nil{t.Fatal(err)}
	if _,err=db.AddVoucher(ctx,batch.ID,"PKG-ONLY-POSA");err!=nil{t.Fatal(err)}
	if _,err=db.FinishVoucherBatch(ctx,batch.ID,0,"");err!=nil{t.Fatal(err)}
	if _,err=db.CreateCardPackageSale(ctx,"CPOS-TEST-0004",posB.ID,packageID,1,"cash");!errors.Is(err,ErrCardPackageSaleStockUnavailable){
		t.Fatalf("other POS stock sale error=%v, want ErrCardPackageSaleStockUnavailable",err)
	}
	if _,err=db.CreateCardPackageSale(ctx,"CPOS-TEST-0005",posA.ID,otherPackageID,1,"cash");!errors.Is(err,ErrCardPackageSaleStockUnavailable){
		t.Fatalf("other package stock sale error=%v, want ErrCardPackageSaleStockUnavailable",err)
	}
	var available int
	if err:=db.QueryRowContext(ctx,"SELECT count(*) FROM vouchers WHERE username='PKG-ONLY-POSA' AND status='available'").Scan(&available);err!=nil{t.Fatal(err)}
	if available!=1{t.Fatalf("voucher owned by POS A changed after rejected sales: available=%d",available)}
}


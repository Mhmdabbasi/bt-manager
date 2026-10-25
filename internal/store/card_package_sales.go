package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrCardPackageSaleNotFound = errors.New("card package sale not found")
	ErrCardPackageSaleNotAllowed = errors.New("card package is not allowed at this point of sale")
	ErrCardPackageSaleStockUnavailable = errors.New("insufficient assigned voucher stock for this package")
)

type CardPackageSale struct {
	ID int64 `json:"id"`
	Reference string `json:"reference"`
	PointOfSaleID int64 `json:"pointOfSaleId"`
	PackageID int64 `json:"packageId"`
	PackageName string `json:"packageName"`
	TotalMinor int64 `json:"totalMinor"`
	CurrencyCode string `json:"currencyCode"`
	PaymentMethod string `json:"paymentMethod"`
	CreatedAt string `json:"createdAt"`
}

type CardPackageSaleItem struct {
	ID int64 `json:"id"`
	VoucherID int64 `json:"voucherId"`
	BatchID int64 `json:"batchId"`
	Username string `json:"username"`
	PackageID int64 `json:"packageId"`
	PackageName string `json:"packageName"`
	UnitPriceMinor int64 `json:"unitPriceMinor"`
	CurrencyCode string `json:"currencyCode"`
}

type CardPackageSaleResult struct {
	Sale CardPackageSale `json:"sale"`
	Items []CardPackageSaleItem `json:"items"`
}

func (db *DB) CreateCardPackageSale(ctx context.Context, reference string, pointOfSaleID, packageID int64, quantity int, paymentMethod string) (CardPackageSaleResult, error) {
	reference = strings.TrimSpace(reference)
	paymentMethod = strings.TrimSpace(paymentMethod)
	if reference == "" || len(reference) > 120 || pointOfSaleID < 1 || packageID < 1 ||
		quantity < 1 || quantity > MaxPOSSaleQuantity || !isSupportedPaymentMethod(paymentMethod) {
		return CardPackageSaleResult{}, ErrInvalidPOSSale
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil { return CardPackageSaleResult{}, err }
	defer tx.Rollback()

	var existingID, existingPOS, existingPackage int64
	var existingPayment string
	err = tx.QueryRowContext(ctx, `SELECT id,point_of_sale_id,package_id,payment_method
		FROM card_package_sales WHERE reference=?`, reference).
		Scan(&existingID, &existingPOS, &existingPackage, &existingPayment)
	if err == nil {
		var itemCount int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM card_package_sale_items WHERE sale_id=?", existingID).Scan(&itemCount); err != nil {
			return CardPackageSaleResult{}, err
		}
		_ = tx.Rollback()
		if itemCount != quantity || existingPOS != pointOfSaleID || existingPackage != packageID || existingPayment != paymentMethod {
			return CardPackageSaleResult{}, ErrInvalidPOSSale
		}
		return db.GetCardPackageSale(ctx, existingID)
	}
	if !errors.Is(err, sql.ErrNoRows) { return CardPackageSaleResult{}, err }

	var currency, packageName, serviceSystem string
	var priceMinor int64
	var posActive, packageActive, enabled int
	err = tx.QueryRowContext(ctx, `SELECT p.active,cp.active,cp.name,cp.service_system,cp.user_price_minor,
		CASE WHEN p.package_selection_mode='all' THEN COALESCE(rule.is_enabled,1) ELSE COALESCE(rule.is_enabled,0) END
		FROM points_of_sale p CROSS JOIN card_packages cp
		LEFT JOIN point_of_sale_packages rule ON rule.point_of_sale_id=p.id AND rule.package_id=cp.id
		WHERE p.id=? AND cp.id=?`, pointOfSaleID, packageID).
		Scan(&posActive, &packageActive, &packageName, &serviceSystem, &priceMinor, &enabled)
	if errors.Is(err, sql.ErrNoRows) { return CardPackageSaleResult{}, ErrCardPackageSaleNotAllowed }
	if err != nil { return CardPackageSaleResult{}, err }
	if posActive != 1 || packageActive != 1 || enabled != 1 || serviceSystem != "hotspot" ||
		priceMinor < 1 || priceMinor > MaxVoucherProductPriceMinor {
		return CardPackageSaleResult{}, ErrCardPackageSaleNotAllowed
	}
	if err := tx.QueryRowContext(ctx, "SELECT value FROM app_settings WHERE key='currency_code'").Scan(&currency); err != nil {
		return CardPackageSaleResult{}, err
	}
	total := priceMinor * int64(quantity)
	if total <= 0 { return CardPackageSaleResult{}, ErrInvalidPOSSale }

	rows, err := tx.QueryContext(ctx, `SELECT id,batch_id,username FROM vouchers
		WHERE package_id=? AND point_of_sale_id=? AND status='available'
		ORDER BY id LIMIT ?`, packageID, pointOfSaleID, quantity)
	if err != nil { return CardPackageSaleResult{}, err }
	selected := make([]struct { id, batchID int64; username string }, 0, quantity)
	for rows.Next() {
		var v struct { id, batchID int64; username string }
		if err := rows.Scan(&v.id, &v.batchID, &v.username); err != nil { _ = rows.Close(); return CardPackageSaleResult{}, err }
		selected = append(selected, v)
	}
	if err := rows.Err(); err != nil { _ = rows.Close(); return CardPackageSaleResult{}, err }
	_ = rows.Close()
	if len(selected) < quantity { return CardPackageSaleResult{}, ErrCardPackageSaleStockUnavailable }

	res, err := tx.ExecContext(ctx, `INSERT INTO card_package_sales
		(reference,point_of_sale_id,package_id,total_minor,currency_code,payment_method)
		VALUES(?,?,?,?,?,?)`, reference, pointOfSaleID, packageID, total, currency, paymentMethod)
	if err != nil { return CardPackageSaleResult{}, fmt.Errorf("create card package sale: %w", err) }
	saleID, err := res.LastInsertId()
	if err != nil { return CardPackageSaleResult{}, err }

	items := make([]CardPackageSaleItem, 0, quantity)
	soldByBatch := make(map[int64]int)
	for _, v := range selected {
		itemRes, err := tx.ExecContext(ctx, `INSERT INTO card_package_sale_items
			(sale_id,voucher_id,batch_id,package_id,username,package_name,unit_price_minor,currency_code)
			VALUES(?,?,?,?,?,?,?,?)`, saleID, v.id, v.batchID, packageID, v.username, packageName, priceMinor, currency)
		if err != nil { return CardPackageSaleResult{}, fmt.Errorf("record card package sale item: %w", err) }
		itemID, err := itemRes.LastInsertId()
		if err != nil { return CardPackageSaleResult{}, err }
		updated, err := tx.ExecContext(ctx, `UPDATE vouchers SET status='sold',sold_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')
			WHERE id=? AND package_id=? AND point_of_sale_id=? AND status='available'`, v.id, packageID, pointOfSaleID)
		if err != nil { return CardPackageSaleResult{}, err }
		affected, err := updated.RowsAffected()
		if err != nil { return CardPackageSaleResult{}, err }
		if affected != 1 { return CardPackageSaleResult{}, ErrCardPackageSaleStockUnavailable }
		soldByBatch[v.batchID]++
		items = append(items, CardPackageSaleItem{ID:itemID,VoucherID:v.id,BatchID:v.batchID,Username:v.username,
			PackageID:packageID,PackageName:packageName,UnitPriceMinor:priceMinor,CurrencyCode:currency})
	}
	for batchID, count := range soldByBatch {
		if _, err := tx.ExecContext(ctx, `INSERT INTO voucher_stock_movements
			(reference,movement_type,batch_id,package_id,from_point_of_sale_id,quantity,note)
			VALUES(?,'sale',?,?,?,?, 'Point-of-sale package sale')`,
			fmt.Sprintf("%s-SALE-%d", reference, batchID), batchID, packageID, pointOfSaleID, count); err != nil {
			return CardPackageSaleResult{}, fmt.Errorf("record package sale stock movement: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO financial_transactions(reference,transaction_type,amount_minor,currency_code)
		VALUES(?,'sale',?,?)`, reference, total, currency); err != nil {
		return CardPackageSaleResult{}, fmt.Errorf("record package sale financial transaction: %w", err)
	}
	var createdAt string
	if err := tx.QueryRowContext(ctx, "SELECT created_at FROM card_package_sales WHERE id=?", saleID).Scan(&createdAt); err != nil {
		return CardPackageSaleResult{}, err
	}
	if err := tx.Commit(); err != nil { return CardPackageSaleResult{}, err }
	return CardPackageSaleResult{Sale:CardPackageSale{ID:saleID,Reference:reference,PointOfSaleID:pointOfSaleID,
		PackageID:packageID,PackageName:packageName,TotalMinor:total,CurrencyCode:currency,PaymentMethod:paymentMethod,CreatedAt:createdAt},Items:items}, nil
}

func (db *DB) GetCardPackageSale(ctx context.Context, id int64) (CardPackageSaleResult, error) {
	var result CardPackageSaleResult
	err := db.QueryRowContext(ctx, `SELECT s.id,s.reference,s.point_of_sale_id,s.package_id,cp.name,s.total_minor,
		s.currency_code,s.payment_method,s.created_at
		FROM card_package_sales s JOIN card_packages cp ON cp.id=s.package_id WHERE s.id=?`, id).
		Scan(&result.Sale.ID,&result.Sale.Reference,&result.Sale.PointOfSaleID,&result.Sale.PackageID,&result.Sale.PackageName,
			&result.Sale.TotalMinor,&result.Sale.CurrencyCode,&result.Sale.PaymentMethod,&result.Sale.CreatedAt)
	if errors.Is(err,sql.ErrNoRows) { return CardPackageSaleResult{},ErrCardPackageSaleNotFound }
	if err != nil { return CardPackageSaleResult{},err }
	rows,err:=db.QueryContext(ctx,`SELECT id,voucher_id,batch_id,username,package_id,package_name,unit_price_minor,currency_code
		FROM card_package_sale_items WHERE sale_id=? ORDER BY id`,id)
	if err!=nil{return CardPackageSaleResult{},err}
	defer rows.Close()
	result.Items=make([]CardPackageSaleItem,0)
	for rows.Next(){
		var item CardPackageSaleItem
		if err:=rows.Scan(&item.ID,&item.VoucherID,&item.BatchID,&item.Username,&item.PackageID,&item.PackageName,&item.UnitPriceMinor,&item.CurrencyCode);err!=nil{return CardPackageSaleResult{},err}
		result.Items=append(result.Items,item)
	}
	if err:=rows.Err();err!=nil{return CardPackageSaleResult{},err}
	return result,nil
}

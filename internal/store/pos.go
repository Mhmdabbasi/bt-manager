package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

var (
	ErrVoucherProductNotFound = errors.New("voucher product not found")
	ErrPOSSaleNotFound        = errors.New("POS sale not found")
	ErrInsufficientVoucherStock = errors.New("insufficient available voucher stock")
	ErrInvalidPOSSale         = errors.New("invalid POS sale")
)

const MaxPOSSaleQuantity = 200
const MaxVoucherProductPriceMinor int64 = 1_000_000_000_000

type VoucherProduct struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	ProfileName string `json:"profileName"`
	ServerName  string `json:"serverName"`
	PriceMinor  int64  `json:"priceMinor"`
	Active      bool   `json:"active"`
	StockCount  int    `json:"stockCount"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type VoucherProductInput struct {
	Name        string
	ProfileName string
	ServerName  string
	PriceMinor  int64
	Active      bool
}

type POSSale struct {
	ID            int64  `json:"id"`
	Reference     string `json:"reference"`
	TotalMinor    int64  `json:"totalMinor"`
	CurrencyCode  string `json:"currencyCode"`
	PaymentMethod string `json:"paymentMethod"`
	CreatedAt     string `json:"createdAt"`
}

type POSSaleItem struct {
	ID             int64  `json:"id"`
	VoucherID      int64  `json:"voucherId"`
	Username       string `json:"username"`
	ProductID      int64  `json:"productId"`
	ProductName    string `json:"productName"`
	ProfileName    string `json:"profileName"`
	ServerName     string `json:"serverName"`
	UnitPriceMinor int64  `json:"unitPriceMinor"`
	CurrencyCode   string `json:"currencyCode"`
}

type POSSaleResult struct {
	Sale  POSSale      `json:"sale"`
	Items []POSSaleItem `json:"items"`
}

func (db *DB) CreateVoucherProduct(ctx context.Context, input VoucherProductInput) (VoucherProduct, error) {
	input, err := normalizeVoucherProduct(input)
	if err != nil {
		return VoucherProduct{}, err
	}
	result, err := db.ExecContext(ctx, `INSERT INTO voucher_products(name,profile_name,server_name,price_minor,active) VALUES(?,?,?,?,?)`,
		input.Name, input.ProfileName, input.ServerName, input.PriceMinor, boolInt(input.Active))
	if err != nil {
		return VoucherProduct{}, fmt.Errorf("create voucher product: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return VoucherProduct{}, err
	}
	return db.GetVoucherProduct(ctx, id)
}

func (db *DB) UpdateVoucherProduct(ctx context.Context, id int64, input VoucherProductInput) (VoucherProduct, error) {
	if id < 1 {
		return VoucherProduct{}, ErrVoucherProductNotFound
	}
	input, err := normalizeVoucherProduct(input)
	if err != nil {
		return VoucherProduct{}, err
	}
	result, err := db.ExecContext(ctx, `UPDATE voucher_products SET name=?,profile_name=?,server_name=?,price_minor=?,active=?,
		updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?`,
		input.Name, input.ProfileName, input.ServerName, input.PriceMinor, boolInt(input.Active), id)
	if err != nil {
		return VoucherProduct{}, fmt.Errorf("update voucher product: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return VoucherProduct{}, err
	}
	if affected == 0 {
		return VoucherProduct{}, ErrVoucherProductNotFound
	}
	return db.GetVoucherProduct(ctx, id)
}

func (db *DB) GetVoucherProduct(ctx context.Context, id int64) (VoucherProduct, error) {
	var product VoucherProduct
	var active int
	err := db.QueryRowContext(ctx, `SELECT p.id,p.name,p.profile_name,p.server_name,p.price_minor,p.active,
		(SELECT count(*) FROM vouchers v WHERE v.profile_name=p.profile_name AND v.server_name=p.server_name AND v.status='available'),
		p.created_at,p.updated_at FROM voucher_products p WHERE p.id=?`, id).
		Scan(&product.ID, &product.Name, &product.ProfileName, &product.ServerName, &product.PriceMinor, &active,
			&product.StockCount, &product.CreatedAt, &product.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return VoucherProduct{}, ErrVoucherProductNotFound
	}
	product.Active = active == 1
	return product, err
}

func (db *DB) ListVoucherProducts(ctx context.Context, includeInactive bool) ([]VoucherProduct, error) {
	query := `SELECT p.id,p.name,p.profile_name,p.server_name,p.price_minor,p.active,
		(SELECT count(*) FROM vouchers v WHERE v.profile_name=p.profile_name AND v.server_name=p.server_name AND v.status='available'),
		p.created_at,p.updated_at FROM voucher_products p`
	if !includeInactive {
		query += " WHERE p.active=1"
	}
	query += " ORDER BY p.name COLLATE NOCASE"
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	products := make([]VoucherProduct, 0)
	for rows.Next() {
		var product VoucherProduct
		var active int
		if err := rows.Scan(&product.ID, &product.Name, &product.ProfileName, &product.ServerName, &product.PriceMinor,
			&active, &product.StockCount, &product.CreatedAt, &product.UpdatedAt); err != nil {
			return nil, err
		}
		product.Active = active == 1
		products = append(products, product)
	}
	return products, rows.Err()
}

func (db *DB) CreatePOSSale(ctx context.Context, reference string, productID int64, quantity int, paymentMethod string) (POSSaleResult, error) {
	reference = strings.TrimSpace(reference)
	paymentMethod = strings.TrimSpace(paymentMethod)
	if reference == "" || productID < 1 || quantity < 1 || quantity > MaxPOSSaleQuantity ||
		!isSupportedPaymentMethod(paymentMethod) {
		return POSSaleResult{}, ErrInvalidPOSSale
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return POSSaleResult{}, err
	}
	defer tx.Rollback()

	// A retried request with the same sale reference is idempotent. Return the
	// original receipt only when the product, quantity, and payment method match.
	var existingID int64
	var existingPayment string
	err = tx.QueryRowContext(ctx, `SELECT id,payment_method FROM pos_sales WHERE reference=?`, reference).
		Scan(&existingID, &existingPayment)
	if err == nil {
		var itemCount int
		var minProduct, maxProduct sql.NullInt64
		if err := tx.QueryRowContext(ctx, `SELECT count(*),min(product_id),max(product_id) FROM pos_sale_items WHERE sale_id=?`, existingID).
			Scan(&itemCount, &minProduct, &maxProduct); err != nil {
			return POSSaleResult{}, err
		}
		_ = tx.Rollback()
		if itemCount != quantity || !minProduct.Valid || !maxProduct.Valid ||
			minProduct.Int64 != productID || maxProduct.Int64 != productID || existingPayment != paymentMethod {
			return POSSaleResult{}, ErrInvalidPOSSale
		}
		return db.GetPOSSale(ctx, existingID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return POSSaleResult{}, err
	}

	var currency string
	if err := tx.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key='currency_code'`).Scan(&currency); err != nil {
		return POSSaleResult{}, err
	}
	var productName, profileName, serverName string
	var priceMinor int64
	var active int
	if err := tx.QueryRowContext(ctx, `SELECT name,profile_name,server_name,price_minor,active FROM voucher_products WHERE id=?`, productID).
		Scan(&productName, &profileName, &serverName, &priceMinor, &active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return POSSaleResult{}, ErrVoucherProductNotFound
		}
		return POSSaleResult{}, err
	}
	if active != 1 || priceMinor < 1 || priceMinor > MaxVoucherProductPriceMinor {
		return POSSaleResult{}, ErrInvalidPOSSale
	}
	total := priceMinor * int64(quantity)
	if total <= 0 {
		return POSSaleResult{}, ErrInvalidPOSSale
	}

	rows, err := tx.QueryContext(ctx, `SELECT id,username FROM vouchers
		WHERE profile_name=? AND server_name=? AND status='available' ORDER BY id LIMIT ?`, profileName, serverName, quantity)
	if err != nil {
		return POSSaleResult{}, err
	}
	selected := make([]struct {
		id       int64
		username string
	}, 0, quantity)
	for rows.Next() {
		var voucher struct {
			id       int64
			username string
		}
		if err := rows.Scan(&voucher.id, &voucher.username); err != nil {
			_ = rows.Close()
			return POSSaleResult{}, err
		}
		selected = append(selected, voucher)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return POSSaleResult{}, err
	}
	_ = rows.Close()
	if len(selected) < quantity {
		return POSSaleResult{}, ErrInsufficientVoucherStock
	}

	saleResult, err := tx.ExecContext(ctx, `INSERT INTO pos_sales(reference,total_minor,currency_code,payment_method)
		VALUES(?,?,?,?)`, reference, total, currency, paymentMethod)
	if err != nil {
		return POSSaleResult{}, fmt.Errorf("create POS sale: %w", err)
	}
	saleID, err := saleResult.LastInsertId()
	if err != nil {
		return POSSaleResult{}, err
	}
	items := make([]POSSaleItem, 0, len(selected))
	for _, voucher := range selected {
		itemResult, err := tx.ExecContext(ctx, `INSERT INTO pos_sale_items
			(sale_id,voucher_id,product_id,product_name,profile_name,server_name,unit_price_minor,currency_code)
			VALUES(?,?,?,?,?,?,?,?)`, saleID, voucher.id, productID, productName, profileName, serverName, priceMinor, currency)
		if err != nil {
			return POSSaleResult{}, fmt.Errorf("record POS sale item: %w", err)
		}
		itemID, err := itemResult.LastInsertId()
		if err != nil {
			return POSSaleResult{}, err
		}
		update, err := tx.ExecContext(ctx, `UPDATE vouchers SET status='sold',sale_id=?,
			sold_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=? AND status='available'`, saleID, voucher.id)
		if err != nil {
			return POSSaleResult{}, err
		}
		affected, err := update.RowsAffected()
		if err != nil {
			return POSSaleResult{}, err
		}
		if affected != 1 {
			return POSSaleResult{}, ErrInsufficientVoucherStock
		}
		items = append(items, POSSaleItem{
			ID: itemID, VoucherID: voucher.id, Username: voucher.username,
			ProductID: productID, ProductName: productName, ProfileName: profileName, ServerName: serverName,
			UnitPriceMinor: priceMinor, CurrencyCode: currency,
		})
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO financial_transactions(reference,transaction_type,amount_minor,currency_code)
		VALUES(?,'sale',?,?)`, reference, total, currency); err != nil {
		return POSSaleResult{}, fmt.Errorf("record financial transaction: %w", err)
	}
	sale := POSSale{ID: saleID, Reference: reference, TotalMinor: total, CurrencyCode: currency, PaymentMethod: paymentMethod}
	if err := tx.QueryRowContext(ctx, `SELECT created_at FROM pos_sales WHERE id=?`, saleID).Scan(&sale.CreatedAt); err != nil {
		return POSSaleResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return POSSaleResult{}, err
	}
	return POSSaleResult{Sale: sale, Items: items}, nil
}

func (db *DB) ListPOSSales(ctx context.Context, limit int) ([]POSSale, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	rows, err := db.QueryContext(ctx, `SELECT id,reference,total_minor,currency_code,payment_method,created_at
		FROM pos_sales ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	sales := make([]POSSale, 0)
	for rows.Next() {
		var sale POSSale
		if err := rows.Scan(&sale.ID, &sale.Reference, &sale.TotalMinor, &sale.CurrencyCode,
			&sale.PaymentMethod, &sale.CreatedAt); err != nil {
			return nil, err
		}
		sales = append(sales, sale)
	}
	return sales, rows.Err()
}

func (db *DB) GetPOSSale(ctx context.Context, id int64) (POSSaleResult, error) {
	var result POSSaleResult
	err := db.QueryRowContext(ctx, `SELECT id,reference,total_minor,currency_code,payment_method,created_at
		FROM pos_sales WHERE id=?`, id).
		Scan(&result.Sale.ID, &result.Sale.Reference, &result.Sale.TotalMinor,
			&result.Sale.CurrencyCode, &result.Sale.PaymentMethod, &result.Sale.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return POSSaleResult{}, ErrPOSSaleNotFound
	}
	if err != nil {
		return POSSaleResult{}, err
	}
	rows, err := db.QueryContext(ctx, `SELECT i.id,i.voucher_id,v.username,i.product_id,i.product_name,i.profile_name,i.server_name,i.unit_price_minor,i.currency_code
		FROM pos_sale_items i JOIN vouchers v ON v.id=i.voucher_id WHERE i.sale_id=? ORDER BY i.id`, id)
	if err != nil {
		return POSSaleResult{}, err
	}
	defer rows.Close()
	result.Items = make([]POSSaleItem, 0)
	for rows.Next() {
		var item POSSaleItem
		if err := rows.Scan(&item.ID, &item.VoucherID, &item.Username, &item.ProductID, &item.ProductName,
			&item.ProfileName, &item.ServerName, &item.UnitPriceMinor, &item.CurrencyCode); err != nil {
			return POSSaleResult{}, err
		}
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		return POSSaleResult{}, err
	}
	return result, nil
}

func normalizeVoucherProduct(input VoucherProductInput) (VoucherProductInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.ProfileName = strings.TrimSpace(input.ProfileName)
	input.ServerName = strings.TrimSpace(input.ServerName)
	if input.ServerName == "" { input.ServerName = "all" }
	if input.Name == "" || input.ProfileName == "" || len([]rune(input.Name)) > 80 ||
		len([]rune(input.ProfileName)) > 64 || len([]rune(input.ServerName)) > 64 || input.ServerName == "" || input.PriceMinor < 1 ||
		input.PriceMinor > MaxVoucherProductPriceMinor || hasControlCharacters(input.Name) ||
		hasControlCharacters(input.ProfileName) || hasControlCharacters(input.ServerName) {
		return VoucherProductInput{}, ErrInvalidPOSSale
	}
	return input, nil
}

func hasControlCharacters(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func isSupportedPaymentMethod(value string) bool {
	switch value {
	case "cash", "transfer", "card", "other":
		return true
	default:
		return false
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

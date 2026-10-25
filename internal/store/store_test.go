package store

import (
	"context"
	"path/filepath"
	"testing"
)
func TestOpenAndMigrate(t *testing.T){
	db,err:=Open(filepath.Join(t.TempDir(),"test.db"));if err!=nil{t.Fatal(err)};defer db.Close()
	s,err:=db.GetSettings(context.Background());if err!=nil{t.Fatal(err)}
	if s.CurrencyCode!="YER"||s.CurrencySymbol!="ر.ي"{t.Fatalf("unexpected defaults: %+v",s)}
	var n int;if err:=db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&n);err!=nil{t.Fatal(err)};if n!=11{t.Fatalf("migration count=%d, want 11",n)}
}
func TestCurrencyLocksAfterTransaction(t *testing.T){
	db,err:=Open(filepath.Join(t.TempDir(),"test.db"));if err!=nil{t.Fatal(err)};defer db.Close()
	if _,err:=db.Exec(`INSERT INTO financial_transactions(reference,transaction_type,amount_minor,currency_code) VALUES('POS-1','sale',100,'YER')`);err!=nil{t.Fatal(err)}
	s,err:=db.GetSettings(context.Background());if err!=nil{t.Fatal(err)};s.CurrencyCode="SAR";s.CurrencySymbol="ر.س"
	if err:=db.SaveSettings(context.Background(),s);err!=ErrCurrencyLocked{t.Fatalf("expected ErrCurrencyLocked; got %v",err)}
}

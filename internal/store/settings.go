package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrCurrencyLocked=errors.New("currency cannot be changed after financial transactions exist")
type Settings struct {
	BusinessName string `json:"businessName"`
	CurrencyCode string `json:"currencyCode"`
	CurrencySymbol string `json:"currencySymbol"`
	SetupComplete bool `json:"setupComplete"`
}
func(db *DB)GetSettings(ctx context.Context)(Settings,error){
	v:=map[string]string{};rows,err:=db.QueryContext(ctx,`SELECT key,value FROM app_settings`);if err!=nil{return Settings{},err};defer rows.Close()
	for rows.Next(){var k,val string;if err:=rows.Scan(&k,&val);err!=nil{return Settings{},err};v[k]=val};if err:=rows.Err();err!=nil{return Settings{},err}
	return Settings{BusinessName:v["business_name"],CurrencyCode:v["currency_code"],CurrencySymbol:v["currency_symbol"],SetupComplete:strings.EqualFold(v["setup_complete"],"true")},nil
}
func(db *DB)SaveSettings(ctx context.Context,s Settings)error{
	tx,err:=db.BeginTx(ctx,nil);if err!=nil{return err};defer tx.Rollback()
	var n int;if err:=tx.QueryRowContext(ctx,`SELECT count(*) FROM financial_transactions`).Scan(&n);err!=nil{return err}
	var old string;if err:=tx.QueryRowContext(ctx,`SELECT value FROM app_settings WHERE key='currency_code'`).Scan(&old);err!=nil&&!errors.Is(err,sql.ErrNoRows){return err}
	if n>0&&!strings.EqualFold(old,s.CurrencyCode){return ErrCurrencyLocked}
	values:=map[string]string{"business_name":strings.TrimSpace(s.BusinessName),"currency_code":strings.ToUpper(strings.TrimSpace(s.CurrencyCode)),"currency_symbol":strings.TrimSpace(s.CurrencySymbol),"setup_complete":fmt.Sprintf("%t",s.SetupComplete)}
	for k,v:=range values{if _,err:=tx.ExecContext(ctx,`INSERT INTO app_settings(key,value,updated_at) VALUES(?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now')) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`,k,v);err!=nil{return err}}
	return tx.Commit()
}

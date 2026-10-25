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
	ErrPointOfSaleNotFound             = errors.New("point of sale not found")
	ErrInvalidPointOfSale              = errors.New("invalid point of sale")
	ErrPointOfSaleCodeExists           = errors.New("point of sale code already exists")
	ErrInvalidPointOfSalePackageRules  = errors.New("invalid point of sale package rules")
	ErrPointOfSalePackageNotFound      = errors.New("point of sale package not found")
)

const MaxPointOfSaleRules = 10000

type PointOfSale struct {
	ID                   int64  `json:"id"`
	Code                 string `json:"code"`
	Name                 string `json:"name"`
	Phone                string `json:"phone"`
	Address              string `json:"address"`
	Notes                string `json:"notes"`
	PackageSelectionMode string `json:"packageSelectionMode"`
	Active               bool   `json:"active"`
	PackageRuleCount     int    `json:"packageRuleCount"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
}

type PointOfSaleInput struct {
	Code                 string
	Name                 string
	Phone                string
	Address              string
	Notes                string
	PackageSelectionMode string
	Active               bool
}

type PointOfSalePackageRule struct {
	PackageID          int64   `json:"packageId"`
	PackageName        string  `json:"packageName"`
	ServiceSystem      string  `json:"serviceSystem"`
	ServiceProfileName string  `json:"serviceProfileName"`
	UserPriceMinor     int64   `json:"userPriceMinor"`
	DealerPriceMinor   int64   `json:"dealerPriceMinor"`
	PackageActive      bool    `json:"packageActive"`
	IsEnabled          bool    `json:"isEnabled"`
	HasRule            bool    `json:"hasRule"`
	PrefixOverride     *string `json:"prefixOverride"`
	TemplateIDOverride *int64  `json:"templateIdOverride"`
}

type PointOfSalePackageRuleInput struct {
	PackageID          int64   `json:"packageId"`
	IsEnabled          bool    `json:"isEnabled"`
	PrefixOverride     *string `json:"prefixOverride"`
	TemplateIDOverride *int64  `json:"templateIdOverride"`
}

const pointOfSaleSelect = `SELECT
	p.id,p.code,p.name,p.phone,p.address,p.notes,p.package_selection_mode,p.active,
	(SELECT count(*) FROM point_of_sale_packages pr WHERE pr.point_of_sale_id=p.id),
	p.created_at,p.updated_at
	FROM points_of_sale p`

func (db *DB) CreatePointOfSale(ctx context.Context, input PointOfSaleInput) (PointOfSale, error) {
	input, err := normalizePointOfSale(input)
	if err != nil {
		return PointOfSale{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return PointOfSale{}, err
	}
	defer tx.Rollback()

	if err := ensureUniquePointOfSaleCode(ctx, tx, input.Code, 0); err != nil {
		return PointOfSale{}, err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO points_of_sale
		(code,name,phone,address,notes,package_selection_mode,active)
		VALUES(?,?,?,?,?,?,?)`,
		input.Code,input.Name,input.Phone,input.Address,input.Notes,input.PackageSelectionMode,boolInt(input.Active))
	if err != nil {
		return PointOfSale{}, fmt.Errorf("create point of sale: %w",err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return PointOfSale{}, err
	}
	if err := tx.Commit(); err != nil {
		return PointOfSale{}, err
	}
	return db.GetPointOfSale(ctx,id)
}

func (db *DB) UpdatePointOfSale(ctx context.Context, id int64, input PointOfSaleInput) (PointOfSale, error) {
	if id < 1 {
		return PointOfSale{}, ErrPointOfSaleNotFound
	}
	input, err := normalizePointOfSale(input)
	if err != nil {
		return PointOfSale{}, err
	}
	tx, err := db.BeginTx(ctx,nil)
	if err != nil {
		return PointOfSale{},err
	}
	defer tx.Rollback()

	var exists int
	if err:=tx.QueryRowContext(ctx,"SELECT count(*) FROM points_of_sale WHERE id=?",id).Scan(&exists);err!=nil{return PointOfSale{},err}
	if exists==0{return PointOfSale{},ErrPointOfSaleNotFound}
	if err:=ensureUniquePointOfSaleCode(ctx,tx,input.Code,id);err!=nil{return PointOfSale{},err}
	_,err=tx.ExecContext(ctx,`UPDATE points_of_sale SET
		code=?,name=?,phone=?,address=?,notes=?,package_selection_mode=?,active=?,
		updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?`,
		input.Code,input.Name,input.Phone,input.Address,input.Notes,input.PackageSelectionMode,boolInt(input.Active),id)
	if err!=nil{return PointOfSale{},fmt.Errorf("update point of sale: %w",err)}
	if err:=tx.Commit();err!=nil{return PointOfSale{},err}
	return db.GetPointOfSale(ctx,id)
}

func (db *DB) ArchivePointOfSale(ctx context.Context, id int64) (PointOfSale,error) {
	if id<1{return PointOfSale{},ErrPointOfSaleNotFound}
	result,err:=db.ExecContext(ctx,`UPDATE points_of_sale SET active=0,
		updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?`,id)
	if err!=nil{return PointOfSale{},fmt.Errorf("archive point of sale: %w",err)}
	affected,err:=result.RowsAffected();if err!=nil{return PointOfSale{},err}
	if affected==0{return PointOfSale{},ErrPointOfSaleNotFound}
	return db.GetPointOfSale(ctx,id)
}

func (db *DB) GetPointOfSale(ctx context.Context,id int64)(PointOfSale,error){
	if id<1{return PointOfSale{},ErrPointOfSaleNotFound}
	pos,err:=scanPointOfSale(db.QueryRowContext(ctx,pointOfSaleSelect+" WHERE p.id=?",id))
	if errors.Is(err,sql.ErrNoRows){return PointOfSale{},ErrPointOfSaleNotFound}
	return pos,err
}

func (db *DB) ListPointsOfSale(ctx context.Context,includeInactive bool)([]PointOfSale,error){
	query:=pointOfSaleSelect
	if !includeInactive{query+=" WHERE p.active=1"}
	query+=" ORDER BY p.active DESC,p.name COLLATE NOCASE,p.id"
	rows,err:=db.QueryContext(ctx,query);if err!=nil{return nil,err};defer rows.Close()
	items:=make([]PointOfSale,0)
	for rows.Next(){pos,err:=scanPointOfSale(rows);if err!=nil{return nil,err};items=append(items,pos)}
	if err:=rows.Err();err!=nil{return nil,err}
	return items,nil
}

func (db *DB) ListPointOfSalePackageRules(ctx context.Context,posID int64,includeInactivePackages bool)([]PointOfSalePackageRule,error){
	if posID<1{return nil,ErrPointOfSaleNotFound}
	var mode string
	if err:=db.QueryRowContext(ctx,"SELECT package_selection_mode FROM points_of_sale WHERE id=?",posID).Scan(&mode);errors.Is(err,sql.ErrNoRows){return nil,ErrPointOfSaleNotFound}else if err!=nil{return nil,err}
	query:=`SELECT cp.id,cp.name,cp.service_system,sp.name,cp.user_price_minor,cp.dealer_price_minor,
		cp.active,CASE WHEN p.package_selection_mode='all' THEN COALESCE(rule.is_enabled,1) ELSE COALESCE(rule.is_enabled,0) END,
		CASE WHEN rule.package_id IS NULL THEN 0 ELSE 1 END,rule.prefix_override,rule.template_id_override
		FROM card_packages cp
		JOIN service_profiles sp ON sp.id=cp.service_profile_id
		JOIN points_of_sale p ON p.id=?
		LEFT JOIN point_of_sale_packages rule ON rule.package_id=cp.id AND rule.point_of_sale_id=p.id`
	if !includeInactivePackages{query+=" WHERE cp.active=1"}
	query+=" ORDER BY cp.active DESC,cp.sort_order,cp.name COLLATE NOCASE,cp.id"
	rows,err:=db.QueryContext(ctx,query,posID);if err!=nil{return nil,err};defer rows.Close()
	rules:=make([]PointOfSalePackageRule,0)
	for rows.Next(){
		var rule PointOfSalePackageRule
		var active,enabled,hasRule int
		var prefix sql.NullString
		var templateID sql.NullInt64
		if err:=rows.Scan(&rule.PackageID,&rule.PackageName,&rule.ServiceSystem,&rule.ServiceProfileName,
			&rule.UserPriceMinor,&rule.DealerPriceMinor,&active,&enabled,&hasRule,&prefix,&templateID);err!=nil{return nil,err}
		rule.PackageActive=active==1;rule.IsEnabled=enabled==1;rule.HasRule=hasRule==1
		if prefix.Valid{v:=prefix.String;rule.PrefixOverride=&v}
		if templateID.Valid{v:=templateID.Int64;rule.TemplateIDOverride=&v}
		rules=append(rules,rule)
	}
	if err:=rows.Err();err!=nil{return nil,err}
	return rules,nil
}

// ReplacePointOfSalePackageRules makes the assignment update atomic. In "all" mode, an
// absent rule means allowed; in "selected" mode, an absent rule means disallowed.
func (db *DB) ReplacePointOfSalePackageRules(ctx context.Context,posID int64,mode string,inputs []PointOfSalePackageRuleInput)([]PointOfSalePackageRule,error){
	mode=strings.ToLower(strings.TrimSpace(mode))
	if posID<1|| (mode!="all"&&mode!="selected")||len(inputs)>MaxPointOfSaleRules{
		return nil,ErrInvalidPointOfSalePackageRules
	}
	tx,err:=db.BeginTx(ctx,nil);if err!=nil{return nil,err};defer tx.Rollback()
	var exists int
	if err:=tx.QueryRowContext(ctx,"SELECT count(*) FROM points_of_sale WHERE id=?",posID).Scan(&exists);err!=nil{return nil,err}
	if exists==0{return nil,ErrPointOfSaleNotFound}
	seen:=make(map[int64]struct{},len(inputs))
	for i:=range inputs{
		item:=&inputs[i]
		if item.PackageID<1 {return nil,ErrInvalidPointOfSalePackageRules}
		if _,ok:=seen[item.PackageID];ok{return nil,ErrInvalidPointOfSalePackageRules};seen[item.PackageID]=struct{}{}
		var packageExists int
		if err:=tx.QueryRowContext(ctx,"SELECT count(*) FROM card_packages WHERE id=?",item.PackageID).Scan(&packageExists);err!=nil{return nil,err}
		if packageExists==0{return nil,ErrPointOfSalePackageNotFound}
		if item.PrefixOverride!=nil{
			value:=strings.TrimSpace(*item.PrefixOverride)
			if value==""{item.PrefixOverride=nil}else{
				if len(value)>32||!validCardPackagePrefix(value){return nil,ErrInvalidPointOfSalePackageRules}
				item.PrefixOverride=&value
			}
		}
		if item.TemplateIDOverride!=nil{
			var valid int
			err:=tx.QueryRowContext(ctx,`SELECT count(*) FROM card_templates t WHERE t.id=? AND t.package_id=?
				AND (t.scope_kind='global' OR t.point_of_sale_id=?)`,*item.TemplateIDOverride,item.PackageID,posID).Scan(&valid)
			if err!=nil{return nil,err};if valid==0{return nil,ErrInvalidPointOfSalePackageRules}
		}
	}
	_,err=tx.ExecContext(ctx,"UPDATE points_of_sale SET package_selection_mode=?,updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE id=?",mode,posID)
	if err!=nil{return nil,fmt.Errorf("update point of sale package selection mode: %w",err)}
	if _,err=tx.ExecContext(ctx,"DELETE FROM point_of_sale_packages WHERE point_of_sale_id=?",posID);err!=nil{return nil,err}
	defaultEnabled:=mode=="all"
	for _,item:=range inputs{
		needsRule:=item.IsEnabled!=defaultEnabled||item.PrefixOverride!=nil||item.TemplateIDOverride!=nil
		if !needsRule{continue}
		_,err=tx.ExecContext(ctx,`INSERT INTO point_of_sale_packages
			(point_of_sale_id,package_id,is_enabled,prefix_override,template_id_override,updated_at)
			VALUES(?,?,?,?,?,strftime('%Y-%m-%dT%H:%M:%fZ','now'))`,
			posID,item.PackageID,boolInt(item.IsEnabled),nullableString(item.PrefixOverride),nullableInt64(item.TemplateIDOverride))
		if err!=nil{return nil,fmt.Errorf("save point of sale package rule: %w",err)}
	}
	if err:=tx.Commit();err!=nil{return nil,err}
	return db.ListPointOfSalePackageRules(ctx,posID,true)
}

func normalizePointOfSale(input PointOfSaleInput)(PointOfSaleInput,error){
	input.Code=strings.ToUpper(strings.TrimSpace(input.Code))
	input.Name=strings.TrimSpace(input.Name)
	input.Phone=strings.TrimSpace(input.Phone)
	input.Address=strings.TrimSpace(input.Address)
	input.Notes=strings.TrimSpace(input.Notes)
	input.PackageSelectionMode=strings.ToLower(strings.TrimSpace(input.PackageSelectionMode))
	if input.PackageSelectionMode==""{input.PackageSelectionMode="all"}
	if input.Code==""||len(input.Code)>40||!validPointOfSaleCode(input.Code)||
		input.Name==""||len([]rune(input.Name))>120||len([]rune(input.Phone))>40||
		len([]rune(input.Address))>240||len([]rune(input.Notes))>1000||
		(input.PackageSelectionMode!="all"&&input.PackageSelectionMode!="selected")||
		hasControlCharacters(input.Name)||hasControlCharacters(input.Phone)||hasControlCharacters(input.Address)||hasControlCharacters(input.Notes){
		return PointOfSaleInput{},ErrInvalidPointOfSale
	}
	return input,nil
}
func validPointOfSaleCode(value string)bool{
	for _,r:=range value{
		if (r>='A'&&r<='Z')||(r>='0'&&r<='9')||r=='_'||r=='-'{continue}
		return false
	}
	return true
}
func ensureUniquePointOfSaleCode(ctx context.Context,q cardPackageQueryer,code string,exceptID int64)error{
	var count int
	err:=q.QueryRowContext(ctx,"SELECT count(*) FROM points_of_sale WHERE code=? AND id<>?",code,exceptID).Scan(&count)
	if err!=nil{return err};if count>0{return ErrPointOfSaleCodeExists};return nil
}
type pointOfSaleRowScanner interface{Scan(dest ...any)error}
func scanPointOfSale(row pointOfSaleRowScanner)(PointOfSale,error){
	var pos PointOfSale;var active int
	err:=row.Scan(&pos.ID,&pos.Code,&pos.Name,&pos.Phone,&pos.Address,&pos.Notes,&pos.PackageSelectionMode,&active,&pos.PackageRuleCount,&pos.CreatedAt,&pos.UpdatedAt)
	if err!=nil{return PointOfSale{},err}
	pos.Active=active==1
	return pos,nil
}
func nullableString(value *string)any{if value==nil{return nil};return *value}
func nullableInt64(value *int64)any{if value==nil{return nil};return *value}
func hasPointOfSaleControlCharacters(value string)bool{for _,r:=range value{if unicode.IsControl(r){return true}};return false}

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mhmdabbasi/bt-manager/internal/app"
	"github.com/Mhmdabbasi/bt-manager/internal/store"
)

func pointOfSaleTestHandler(t *testing.T)(http.Handler,*store.DB){
	t.Helper()
	db,err:=store.Open(filepath.Join(t.TempDir(),"pos-api.db"))
	if err!=nil{t.Fatal(err)}
	t.Cleanup(func(){_ = db.Close()})
	logger:=slog.New(slog.NewTextHandler(&bytes.Buffer{},nil))
	return New(app.New(db,logger),logger),db
}

func TestPointOfSaleAPIListCreateUpdateAndArchive(t *testing.T){
	handler,_:=pointOfSaleTestHandler(t)
	rec:=httptest.NewRecorder()
	handler.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,"/api/points-of-sale",nil))
	if rec.Code!=http.StatusOK{t.Fatalf("empty list status %d: %s",rec.Code,rec.Body.String())}
	var list struct{Points []store.PointOfSale `json:"pointsOfSale"`}
	if err:=json.Unmarshal(rec.Body.Bytes(),&list);err!=nil{t.Fatal(err)}
	if list.Points==nil||len(list.Points)!=0{t.Fatalf("expected empty list: %+v",list)}

	rec=httptest.NewRecorder()
	handler.ServeHTTP(rec,jsonRequest(http.MethodPost,"/api/points-of-sale",`{"code":"shop-10","name":"Kiosk 10","phone":"777000010","address":"Market","notes":"test","packageSelectionMode":"all","active":true}`))
	if rec.Code!=http.StatusCreated{t.Fatalf("create status %d: %s",rec.Code,rec.Body.String())}
	var created store.PointOfSale
	if err:=json.Unmarshal(rec.Body.Bytes(),&created);err!=nil{t.Fatal(err)}
	if created.ID<1||created.Code!="SHOP-10"||!created.Active{t.Fatalf("unexpected created POS: %+v",created)}

	rec=httptest.NewRecorder()
	handler.ServeHTTP(rec,jsonRequest(http.MethodPut,"/api/points-of-sale/"+itoa(created.ID),`{"code":"shop-10","name":"Updated Kiosk","phone":"777000011","address":"New Market","notes":"changed","packageSelectionMode":"selected","active":true}`))
	if rec.Code!=http.StatusOK{t.Fatalf("update status %d: %s",rec.Code,rec.Body.String())}
	var updated store.PointOfSale
	if err:=json.Unmarshal(rec.Body.Bytes(),&updated);err!=nil{t.Fatal(err)}
	if updated.Name!="Updated Kiosk"||updated.PackageSelectionMode!="selected"{t.Fatalf("unexpected updated POS: %+v",updated)}

	rec=httptest.NewRecorder()
	handler.ServeHTTP(rec,jsonRequest(http.MethodPost,"/api/points-of-sale/"+itoa(created.ID)+"/archive",`{}`))
	if rec.Code!=http.StatusOK{t.Fatalf("archive status %d: %s",rec.Code,rec.Body.String())}
	var archived store.PointOfSale
	if err:=json.Unmarshal(rec.Body.Bytes(),&archived);err!=nil{t.Fatal(err)}
	if archived.Active{t.Fatalf("POS remains active after archive: %+v",archived)}
}

func TestPointOfSaleAPIValidationAndOriginGuard(t *testing.T){
	handler,_:=pointOfSaleTestHandler(t)
	rec:=httptest.NewRecorder()
	handler.ServeHTTP(rec,jsonRequest(http.MethodPost,"/api/points-of-sale",`{"code":"BAD CODE","name":"Bad","active":true}`))
	if rec.Code!=http.StatusBadRequest{t.Fatalf("invalid code status %d: %s",rec.Code,rec.Body.String())}

	rec=httptest.NewRecorder()
	handler.ServeHTTP(rec,jsonRequest(http.MethodPost,"/api/points-of-sale",`{"code":"SHOP-1","name":"One","active":true}`))
	if rec.Code!=http.StatusCreated{t.Fatalf("setup create status %d: %s",rec.Code,rec.Body.String())}

	rec=httptest.NewRecorder()
	req:=jsonRequest(http.MethodPost,"/api/points-of-sale",`{"code":"SHOP-2","name":"Two","active":true}`)
	req.Header.Set("Origin","https://untrusted.example")
	handler.ServeHTTP(rec,req)
	if rec.Code!=http.StatusForbidden{t.Fatalf("untrusted origin status %d: %s",rec.Code,rec.Body.String())}

	rec=httptest.NewRecorder()
	handler.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,"/api/points-of-sale/nope",nil))
	if rec.Code!=http.StatusBadRequest{t.Fatalf("invalid id status %d: %s",rec.Code,rec.Body.String())}
}

func TestPointOfSalePackageAssignmentAPI(t *testing.T){
	handler,db:=pointOfSaleTestHandler(t)
	ctx:=context.Background()
	_,err:=db.SyncHotSpotProfilesSnapshot(ctx,store.HotSpotProfileSyncInput{
		Address:"192.0.2.41",Port:8728,RouterIdentity:"api-pos-router",RouterOSVersion:"7.20.1",
		Profiles:[]store.ImportedServiceProfile{{SourceID:"*1",Name:"basic"},{SourceID:"*2",Name:"premium"}},
	})
	if err!=nil{t.Fatal(err)}
	pkg1,err:=db.CreateCardPackage(ctx,store.CardPackageInput{Name:"Basic 100",ServiceSystem:"hotspot",ServiceProfileID:1,UserPriceMinor:10000,DealerPriceMinor:8000,Active:true})
	if err!=nil{t.Fatal(err)}
	pkg2,err:=db.CreateCardPackage(ctx,store.CardPackageInput{Name:"Premium 200",ServiceSystem:"hotspot",ServiceProfileID:2,UserPriceMinor:20000,DealerPriceMinor:15000,Active:true})
	if err!=nil{t.Fatal(err)}
	rec:=httptest.NewRecorder()
	handler.ServeHTTP(rec,jsonRequest(http.MethodPost,"/api/points-of-sale",`{"code":"SHOP-41","name":"Kiosk 41","packageSelectionMode":"all","active":true}`))
	if rec.Code!=http.StatusCreated{t.Fatalf("create POS status %d: %s",rec.Code,rec.Body.String())}
	var pos store.PointOfSale
	if err:=json.Unmarshal(rec.Body.Bytes(),&pos);err!=nil{t.Fatal(err)}

	rec=httptest.NewRecorder()
	handler.ServeHTTP(rec,httptest.NewRequest(http.MethodGet,"/api/points-of-sale/"+itoa(pos.ID)+"/packages?includeInactive=true",nil))
	if rec.Code!=http.StatusOK{t.Fatalf("package list status %d: %s",rec.Code,rec.Body.String())}
	var initial struct{Packages []store.PointOfSalePackageRule `json:"packages"`}
	if err:=json.Unmarshal(rec.Body.Bytes(),&initial);err!=nil{t.Fatal(err)}
	if len(initial.Packages)!=2||!initial.Packages[0].IsEnabled||!initial.Packages[1].IsEnabled{t.Fatalf("all-mode package default mismatch: %+v",initial.Packages)}

	prefix:="K41-"
	body:=`{"packageSelectionMode":"selected","packages":[{"packageId":`+itoa(pkg1.ID)+`,"isEnabled":true,"prefixOverride":"K41-"},{"packageId":`+itoa(pkg2.ID)+`,"isEnabled":false}]}`
	rec=httptest.NewRecorder()
	handler.ServeHTTP(rec,jsonRequest(http.MethodPut,"/api/points-of-sale/"+itoa(pos.ID)+"/packages",body))
	if rec.Code!=http.StatusOK{t.Fatalf("replace package rules status %d: %s",rec.Code,rec.Body.String())}
	var changed struct{
		PointOfSale store.PointOfSale `json:"pointOfSale"`
		Packages []store.PointOfSalePackageRule `json:"packages"`
	}
	if err:=json.Unmarshal(rec.Body.Bytes(),&changed);err!=nil{t.Fatal(err)}
	if changed.PointOfSale.PackageSelectionMode!="selected"||len(changed.Packages)!=2{t.Fatalf("unexpected assignment result: %+v",changed)}
	foundAllowed,foundDenied:=false,false
	for _,rule:=range changed.Packages{
		if rule.PackageID==pkg1.ID{foundAllowed=rule.IsEnabled&&rule.PrefixOverride!=nil&&*rule.PrefixOverride==prefix}
		if rule.PackageID==pkg2.ID{foundDenied=!rule.IsEnabled}
	}
	if !foundAllowed||!foundDenied{t.Fatalf("package selection rules not saved: %+v",changed.Packages)}

	rec=httptest.NewRecorder()
	handler.ServeHTTP(rec,jsonRequest(http.MethodPut,"/api/points-of-sale/"+itoa(pos.ID)+"/packages",`{"packageSelectionMode":"bogus","packages":[]}`))
	if rec.Code!=http.StatusBadRequest{t.Fatalf("invalid selection mode status %d: %s",rec.Code,rec.Body.String())}
	rules,err:=db.ListPointOfSalePackageRules(ctx,pos.ID,true)
	if err!=nil{t.Fatal(err)}
	if len(rules)!=2||!rules[0].IsEnabled&&!rules[1].IsEnabled{
		t.Fatalf("invalid assignment request should not change rules: %+v",rules)
	}
	if strings.Contains(rec.Body.String(),"router-secret"){t.Fatal("sensitive content in response")}
}

func itoa(value int64)string{
	if value==0{return "0"}
	const digits="0123456789"
	var buf [20]byte
	i:=len(buf)
	for value>0{i--;buf[i]=digits[value%10];value/=10}
	return string(buf[i:])
}

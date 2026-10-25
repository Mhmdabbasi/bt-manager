package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mhmdabbasi/bt-manager/internal/app"
	"github.com/Mhmdabbasi/bt-manager/internal/store"
)

func TestCardTemplateHTTPCreateAndList(t *testing.T) {
	db,err:=store.Open(filepath.Join(t.TempDir(),"templates.db"))
	if err!=nil { t.Fatal(err) }
	defer db.Close()
	ctx:=context.Background()
	_,err=db.SyncHotSpotProfilesSnapshot(ctx,store.HotSpotProfileSyncInput{
		Address:"192.0.2.71",Port:8728,RouterIdentity:"template-test-router",RouterOSVersion:"7.20.1",
		Profiles:[]store.ImportedServiceProfile{{SourceID:"*1",Name:"TemplateProfile",Properties:map[string]string{}}},
	})
	if err!=nil { t.Fatal(err) }
	pkg,err:=db.CreateCardPackage(ctx,store.CardPackageInput{Name:"Template package",ServiceSystem:"hotspot",ServiceProfileID:1,UserPriceMinor:20000,DealerPriceMinor:15000,Active:true})
	if err!=nil { t.Fatal(err) }
	logger:=slog.New(slog.NewTextHandler(&bytes.Buffer{},nil))
	handler:=New(app.New(db,logger),logger)
	body:=`{"packageId":`+fmt.Sprint(pkg.ID)+`,"name":"A4 template","isDefault":true,"cardWidthMm":50,"cardHeightMm":19.1,"design":[{"id":"username","field":"username","x":5,"y":40,"width":90,"fontSize":12,"color":"#123456","align":"center","bold":true}],"pageLayout":{"columns":4,"rows":15,"marginMm":5,"gapMm":0}}`
	req:=httptest.NewRequest(http.MethodPost,"/api/card-templates",strings.NewReader(body));req.Header.Set("Content-Type","application/json")
	created:=httptest.NewRecorder();handler.ServeHTTP(created,req)
	if created.Code!=http.StatusCreated { t.Fatalf("create status %d: %s",created.Code,created.Body.String()) }
	var result struct { Template struct { ID int64 `json:"id"`; Version int `json:"versionNumber"`; Default bool `json:"isDefault"` } `json:"template"` }
	if err:=json.Unmarshal(created.Body.Bytes(),&result);err!=nil { t.Fatal(err) }
	if result.Template.ID<1||result.Template.Version!=1||!result.Template.Default { t.Fatalf("unexpected template: %+v",result.Template) }
	list:=httptest.NewRecorder();handler.ServeHTTP(list,httptest.NewRequest(http.MethodGet,"/api/card-templates?packageId="+fmt.Sprint(pkg.ID),nil))
	if list.Code!=http.StatusOK||!strings.Contains(list.Body.String(),"A4 template") { t.Fatalf("list status %d: %s",list.Code,list.Body.String()) }
}

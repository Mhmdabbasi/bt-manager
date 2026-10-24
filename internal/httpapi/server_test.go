package httpapi

import (
	"bytes"
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

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	return New(app.New(db, logger), logger)
}

func jsonRequest(method, target, body string) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	testHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var v map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if v["database"] != "ready" {
		t.Fatalf("database: %v", v["database"])
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("security headers missing")
	}
}

func TestSaveSettingsNormalizesCurrency(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"businessName":"شبكة اختبار","currencyCode":"yer","currencySymbol":"ر.ي","setupComplete":false}`
	testHandler(t).ServeHTTP(rec, jsonRequest(http.MethodPut, "/api/settings", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var v map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if v["currencyCode"] != "YER" || v["setupComplete"] != true {
		t.Fatalf("settings: %#v", v)
	}
}

func TestRejectInvalidSettingsAndSecrets(t *testing.T) {
	bodies := []string{
		`{"businessName":"Demo","currencyCode":"YE","currencySymbol":"ر.ي"}`,
		`{"businessName":"Demo","currencyCode":"YER","currencySymbol":"ر.ي","password":"secret"}`,
	}
	for _, body := range bodies {
		rec := httptest.NewRecorder()
		testHandler(t).ServeHTTP(rec, jsonRequest(http.MethodPut, "/api/settings", body))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status %d for %s", rec.Code, body)
		}
	}
}

func TestRouterInvalidInput(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"address":"","port":8728,"username":"admin","password":"secret","useTls":false}`
	testHandler(t).ServeHTTP(rec, jsonRequest(http.MethodPost, "/api/router/test", body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatal("secret leaked")
	}
}

func TestRejectCrossSiteRouterProbe(t *testing.T) {
	rec := httptest.NewRecorder()
	req := jsonRequest(http.MethodPost, "/api/router/test", `{"address":"192.168.88.1","port":8728,"username":"admin","password":"unused","useTls":false}`)
	req.Header.Set("Origin", "https://untrusted.example")
	testHandler(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRejectTextPlainRouterProbe(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/router/test", strings.NewReader(`{"address":"192.168.88.1"}`))
	req.Header.Set("Content-Type", "text/plain")
	testHandler(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAllowLocalDevelopmentOrigin(t *testing.T) {
	rec := httptest.NewRecorder()
	req := jsonRequest(http.MethodPut, "/api/settings", `{"businessName":"Demo","currencyCode":"YER","currencySymbol":"ر.ي"}`)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	testHandler(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHotSpotOverviewRejectsInvalidInput(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"address":"","port":8728,"username":"admin","password":"secret","useTls":false}`
	testHandler(t).ServeHTTP(rec, jsonRequest(http.MethodPost, "/api/router/hotspot/overview", body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatal("secret leaked in error response")
	}
}

func TestHotSpotOverviewRejectsCrossSiteRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	req := jsonRequest(http.MethodPost, "/api/router/hotspot/overview",
		`{"address":"192.168.88.1","port":8728,"username":"admin","password":"unused","useTls":false}`)
	req.Header.Set("Origin", "https://untrusted.example")
	testHandler(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHotSpotUserCreateRejectsMissingName(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"router":{"address":"192.168.88.1","port":8728,"username":"admin","password":"router-secret","useTls":false},"user":{"name":""}}`
	testHandler(t).ServeHTTP(rec, jsonRequest(http.MethodPost, "/api/router/hotspot/users/create", body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "router-secret") {
		t.Fatal("router password leaked in error response")
	}
}

func TestHotSpotUserDeleteRejectsInvalidID(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"router":{"address":"192.168.88.1","port":8728,"username":"admin","password":"router-secret","useTls":false},"id":"*1; /system/reboot"}`
	testHandler(t).ServeHTTP(rec, jsonRequest(http.MethodPost, "/api/router/hotspot/users/delete", body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHotSpotMutationRejectsCrossSiteRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	req := jsonRequest(http.MethodPost, "/api/router/hotspot/users/create",
		`{"router":{"address":"192.168.88.1","port":8728,"username":"admin","password":"unused","useTls":false},"user":{"name":"voucher-1","password":"pass"}}`)
	req.Header.Set("Origin", "https://untrusted.example")
	testHandler(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHotSpotUserCreateRejectsMissingNameWithBlankPassword(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"router":{"address":"192.168.88.1","port":8728,"username":"admin","password":"router-secret","useTls":false},"user":{"name":"","password":""}}`
	testHandler(t).ServeHTTP(rec, jsonRequest(http.MethodPost, "/api/router/hotspot/users/create", body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "router-secret") {
		t.Fatal("router password leaked in error response")
	}
}

func TestHotSpotProfileDeleteRejectsInvalidID(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"router":{"address":"192.168.88.1","port":8728,"username":"admin","password":"unused","useTls":false},"id":"*1; /system/reboot"}`
	testHandler(t).ServeHTTP(rec, jsonRequest(http.MethodPost, "/api/router/hotspot/profiles/delete", body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGenerateVoucherBatchRejectsInvalidCountBeforeRouterConnection(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"router":{"address":"192.168.88.1","port":8728,"username":"admin","password":"router-secret","useTls":false},"prefix":"BT-","count":201,"profile":"default","server":"all","limitUptime":""}`
	testHandler(t).ServeHTTP(rec, jsonRequest(http.MethodPost, "/api/router/hotspot/vouchers/generate", body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "router-secret") {
		t.Fatal("router password leaked in validation error")
	}
}

func TestListVoucherBatchesInitiallyEmpty(t *testing.T) {
	rec := httptest.NewRecorder()
	testHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/vouchers/batches", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Batches []map[string]any `json:"batches"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Batches) != 0 {
		t.Fatalf("expected no batches, got %d", len(payload.Batches))
	}
}

func TestGetVoucherBatchRejectsInvalidID(t *testing.T) {
	rec := httptest.NewRecorder()
	testHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/vouchers/batches/not-a-number", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListPOSProductsInitiallyEmpty(t *testing.T) {
	rec := httptest.NewRecorder()
	testHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/pos/products", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Products []map[string]any `json:"products"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Products) != 0 {
		t.Fatalf("expected empty product catalog, got %d", len(payload.Products))
	}
}

func TestPOSSaleRejectsInvalidQuantityBeforeWriting(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"requestId":"test-sale-0001","productId":1,"quantity":0,"paymentMethod":"cash"}`
	testHandler(t).ServeHTTP(rec, jsonRequest(http.MethodPost, "/api/pos/sales", body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPOSMutationRejectsCrossSiteRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	req := jsonRequest(http.MethodPost, "/api/pos/products", `{"name":"Demo","profileName":"default","priceMinor":20000}`)
	req.Header.Set("Origin", "https://untrusted.example")
	testHandler(t).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPOSSalesListInitiallyEmpty(t *testing.T) {
	rec := httptest.NewRecorder()
	testHandler(t).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/pos/sales", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Sales []map[string]any `json:"sales"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Sales) != 0 {
		t.Fatalf("expected no sales, got %d", len(payload.Sales))
	}
}

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

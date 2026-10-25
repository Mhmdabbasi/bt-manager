package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Mhmdabbasi/bt-manager/internal/app"
	routerosadapter "github.com/Mhmdabbasi/bt-manager/internal/routeros"
	"github.com/Mhmdabbasi/bt-manager/internal/store"
	"github.com/Mhmdabbasi/bt-manager/internal/ui"
)

var currencyCodePattern = regexp.MustCompile(`^[A-Z]{3}$`)

type Server struct {
	app    *app.App
	logger *slog.Logger
}

func New(application *app.App, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{app: application, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "المسار المطلوب غير موجود")
	})
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/settings", s.getSettings)
	mux.HandleFunc("PUT /api/settings", s.putSettings)
	mux.HandleFunc("POST /api/router/test", s.testRouter)
	mux.HandleFunc("POST /api/router/hotspot/overview", s.hotspotOverview)
	mux.HandleFunc("POST /api/router/hotspot/profiles/sync", s.syncHotSpotProfiles)
	mux.HandleFunc("GET /api/service-profiles", s.listServiceProfiles)
	mux.HandleFunc("GET /api/card-packages", s.listCardPackages)
	mux.HandleFunc("POST /api/card-packages", s.createCardPackage)
	mux.HandleFunc("PUT /api/card-packages/{id}", s.updateCardPackage)
	mux.HandleFunc("POST /api/router/usermanager/sync", s.syncUserManager)
	mux.HandleFunc("GET /api/usermanager/profiles", s.listSavedUserManagerProfiles)
	mux.HandleFunc("POST /api/router/hotspot/vouchers/generate", s.generateVouchers)
	mux.HandleFunc("GET /api/vouchers/batches", s.listVoucherBatches)
	mux.HandleFunc("GET /api/vouchers/batches/{id}", s.getVoucherBatch)
	mux.HandleFunc("GET /api/pos/products", s.listVoucherProducts)
	mux.HandleFunc("POST /api/pos/products", s.createVoucherProduct)
	mux.HandleFunc("PUT /api/pos/products/{id}", s.updateVoucherProduct)
	mux.HandleFunc("GET /api/pos/sales", s.listPOSSales)
	mux.HandleFunc("POST /api/pos/sales", s.createPOSSale)
	mux.HandleFunc("GET /api/pos/sales/{id}", s.getPOSSale)
	for _, path := range []string{"/api/router/hotspot/users/create", "/api/router/hotspot/users/update", "/api/router/hotspot/users/state", "/api/router/hotspot/users/delete"} {
		mux.HandleFunc("POST "+path, s.hotSpotUserMutation)
	}
	for _, path := range []string{"/api/router/hotspot/profiles/create", "/api/router/hotspot/profiles/update", "/api/router/hotspot/profiles/delete"} {
		mux.HandleFunc("POST "+path, s.hotSpotProfileMutation)
	}
	mux.Handle("/", ui.Handler())
	return securityHeaders(s.requestLogger(mux))
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}
type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Error: apiError{Code: code, Message: message}})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 32*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("only one JSON value is allowed")
		}
		return err
	}
	return nil
}

// guardLocalMutation blocks browser-originated cross-site requests and
// non-JSON bodies. This is defense-in-depth for loopback mode: the endpoint
// that tests RouterOS must not become a cross-origin LAN-probing primitive.
func guardLocalMutation(w http.ResponseWriter, r *http.Request) bool {
	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "يجب إرسال البيانات بصيغة JSON")
		return false
	}
	if site := strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))); site == "cross-site" {
		writeError(w, http.StatusForbidden, "cross_site_request", "تم رفض الطلب لأنه صادر من موقع آخر")
		return false
	}
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" && !isLoopbackOrigin(origin) {
		writeError(w, http.StatusForbidden, "origin_not_allowed", "مصدر الطلب غير مسموح")
		return false
	}
	return true
}

func isLoopbackOrigin(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "localhost" {
		return parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == ""
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback() && parsed.Path == "" && parsed.RawQuery == "" && parsed.Fragment == ""
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.app.DB.PingContext(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable", "تعذر الوصول إلى قاعدة البيانات المحلية")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok", "app": "bt-manager", "version": "0.1.0-dev", "database": "ready",
	})
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.app.Settings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settings_read_failed", "تعذر قراءة الإعدادات")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	if !guardLocalMutation(w, r) {
		return
	}
	var settings store.Settings
	if err := decodeJSON(w, r, &settings); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "البيانات المرسلة غير صالحة")
		return
	}
	settings.BusinessName = strings.TrimSpace(settings.BusinessName)
	settings.CurrencyCode = strings.ToUpper(strings.TrimSpace(settings.CurrencyCode))
	settings.CurrencySymbol = strings.TrimSpace(settings.CurrencySymbol)
	if settings.BusinessName == "" || len([]rune(settings.BusinessName)) > 120 {
		writeError(w, http.StatusBadRequest, "invalid_business_name", "اسم النشاط مطلوب وبحد أقصى 120 حرفًا")
		return
	}
	if !currencyCodePattern.MatchString(settings.CurrencyCode) {
		writeError(w, http.StatusBadRequest, "invalid_currency_code", "رمز العملة يجب أن يتكون من ثلاثة أحرف، مثل YER")
		return
	}
	if settings.CurrencySymbol == "" || len([]rune(settings.CurrencySymbol)) > 12 {
		writeError(w, http.StatusBadRequest, "invalid_currency_symbol", "رمز العملة مطلوب وبحد أقصى 12 حرفًا")
		return
	}
	settings.SetupComplete = true
	if err := s.app.SaveSettings(r.Context(), settings); err != nil {
		if errors.Is(err, store.ErrCurrencyLocked) {
			writeError(w, http.StatusConflict, "currency_locked", "لا يمكن تغيير العملة بعد تسجيل أول معاملة مالية")
			return
		}
		s.logger.Error("save settings failed", "error", err)
		writeError(w, http.StatusInternalServerError, "settings_save_failed", "تعذر حفظ الإعدادات")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) testRouter(w http.ResponseWriter, r *http.Request) {
	if !guardLocalMutation(w, r) {
		return
	}
	var request routerosadapter.TestRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "بيانات الاتصال غير صالحة")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	info, err := routerosadapter.TestConnection(ctx, request)
	if errors.Is(err, routerosadapter.ErrInvalidRequest) {
		writeError(w, http.StatusBadRequest, "invalid_router_parameters", "أدخل العنوان واسم المستخدم وكلمة المرور ومنفذًا صالحًا")
		return
	}
	if err != nil {
		s.logger.Warn("RouterOS connection test failed", "address", request.Address, "error", err)
		writeError(w, http.StatusBadGateway, "router_connection_failed", "تعذر الاتصال بالراوتر. تحقق من العنوان والمنفذ وبيانات الدخول وإعدادات API/API-SSL.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "connected", "router": info})
}


func (s *Server) hotspotOverview(w http.ResponseWriter, r *http.Request) {
	if !guardLocalMutation(w, r) {
		return
	}
	var request routerosadapter.TestRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "بيانات الاتصال غير صالحة")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	snapshot, err := routerosadapter.HotSpotSnapshot(ctx, request)
	if errors.Is(err, routerosadapter.ErrInvalidRequest) {
		writeError(w, http.StatusBadRequest, "invalid_router_parameters", "أدخل العنوان واسم المستخدم وكلمة المرور ومنفذًا صالحًا")
		return
	}
	if err != nil {
		s.logger.Warn("RouterOS HotSpot read failed", "address", request.Address, "error", err)
		writeError(w, http.StatusBadGateway, "hotspot_read_failed", "تعذرت قراءة بيانات HotSpot. تحقق من صلاحيات API وإعدادات HotSpot على الراوتر.")
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}


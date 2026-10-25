package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	routerosadapter "github.com/Mhmdabbasi/bt-manager/internal/routeros"
	"github.com/Mhmdabbasi/bt-manager/internal/store"
)

type hotSpotProfileSyncRequest struct {
	Router routerosadapter.TestRequest `json:"router"`
}

type hotSpotProfileSyncResponse struct {
	Sync     store.ServiceProfileSyncResult `json:"sync"`
	Router   routerosadapter.RouterInfo     `json:"router"`
	Profiles []store.ServiceProfileRecord  `json:"profiles"`
}

func (s *Server) syncHotSpotProfiles(w http.ResponseWriter, r *http.Request) {
	if !guardLocalMutation(w, r) {
		return
	}
	var req hotSpotProfileSyncRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "بيانات الاتصال بـHotSpot غير صالحة")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	snapshot, err := routerosadapter.ReadHotSpotProfiles(ctx, req.Router)
	if errors.Is(err, routerosadapter.ErrInvalidRequest) {
		writeError(w, http.StatusBadRequest, "invalid_router_parameters", "أدخل عنوان الراوتر وبيانات الدخول ومنفذًا صالحًا")
		return
	}
	if err != nil {
		s.logger.Warn("HotSpot profile import failed", "address", req.Router.Address, "error", err)
		writeError(w, http.StatusBadGateway, "hotspot_profiles_read_failed", "تعذرت قراءة ملفات HotSpot التعريفية؛ لم تُحدّث النسخة المحلية")
		return
	}

	profiles := make([]store.ImportedServiceProfile, 0, len(snapshot.Profiles))
	for _, profile := range snapshot.Profiles {
		profiles = append(profiles, store.ImportedServiceProfile{
			SourceID: profile.SourceID,
			Name: profile.Name,
			Properties: profile.Properties,
		})
	}
	syncResult, err := s.app.DB.SyncHotSpotProfilesSnapshot(ctx, store.HotSpotProfileSyncInput{
		Address: req.Router.Address, Port: effectiveRouterPort(req.Router), UseTLS: req.Router.UseTLS,
		RouterIdentity: snapshot.Router.Identity, RouterOSVersion: snapshot.Router.Version,
		Model: routerModel(snapshot.Router), Profiles: profiles,
	})
	if errors.Is(err, store.ErrInvalidServiceProfileSnapshot) {
		writeError(w, http.StatusUnprocessableEntity, "invalid_hotspot_profile_snapshot", "أعاد الراوتر بيانات ملفات تعريفية غير مكتملة؛ لم تُعتمد المزامنة")
		return
	}
	if err != nil {
		s.logger.Error("save HotSpot profile snapshot failed", "address", req.Router.Address, "error", err)
		writeError(w, http.StatusInternalServerError, "hotspot_profile_sync_failed", "تمت قراءة الملفات التعريفية ولكن تعذر حفظها محليًا")
		return
	}
	saved, err := s.app.DB.ListServiceProfiles(ctx, syncResult.RouterID, "hotspot", true)
	if err != nil {
		s.logger.Error("read saved HotSpot profiles failed", "routerId", syncResult.RouterID, "error", err)
		writeError(w, http.StatusInternalServerError, "hotspot_profiles_saved_read_failed", "تم حفظ المزامنة ولكن تعذر عرض الملفات التعريفية")
		return
	}
	writeJSON(w, http.StatusOK, hotSpotProfileSyncResponse{Sync: syncResult, Router: snapshot.Router, Profiles: saved})
}

func (s *Server) listServiceProfiles(w http.ResponseWriter, r *http.Request) {
	routerID, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("routerId")), 10, 64)
	if err != nil || routerID < 1 {
		writeError(w, http.StatusBadRequest, "invalid_router_id", "معرّف الراوتر المستورد غير صالح")
		return
	}
	system := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("system")))
	if system != "hotspot" && system != "usermanager" {
		writeError(w, http.StatusBadRequest, "invalid_service_system", "حدد نوع المصدر HotSpot أو User Manager")
		return
	}
	includeMissing := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("includeMissing")), "true")
	profiles, err := s.app.DB.ListServiceProfiles(r.Context(), routerID, system, includeMissing)
	if errors.Is(err, store.ErrImportedRouterNotFound) {
		writeError(w, http.StatusNotFound, "imported_router_not_found", "لم يتم استيراد ملفات التعريف لهذا الراوتر بعد")
		return
	}
	if errors.Is(err, store.ErrInvalidServiceProfileSnapshot) {
		writeError(w, http.StatusBadRequest, "invalid_service_system", "نوع المصدر غير صالح")
		return
	}
	if err != nil {
		s.logger.Error("list service profiles failed", "routerId", routerID, "system", system, "error", err)
		writeError(w, http.StatusInternalServerError, "service_profiles_read_failed", "تعذر قراءة ملفات الخدمة المستوردة")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": profiles})
}

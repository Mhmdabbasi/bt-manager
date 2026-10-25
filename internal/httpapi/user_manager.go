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

type userManagerSyncRequest struct {
	Router routerosadapter.TestRequest `json:"router"`
}

type userManagerOverviewResponse struct {
	Sync         store.UserManagerSyncResult  `json:"sync"`
	Router       routerosadapter.RouterInfo   `json:"router"`
	MajorVersion int                          `json:"majorVersion"`
	Profiles     []store.UserManagerProfileRecord     `json:"profiles"`
	Limitations  []store.UserManagerLimitationRecord  `json:"limitations"`
}

func (s *Server) syncUserManager(w http.ResponseWriter, r *http.Request) {
	if !guardLocalMutation(w, r) {
		return
	}
	var req userManagerSyncRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "بيانات الاتصال بـUser Manager غير صالحة")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	snapshot, err := routerosadapter.ReadUserManagerSnapshot(ctx, req.Router)
	if errors.Is(err, routerosadapter.ErrInvalidRequest) {
		writeError(w, http.StatusBadRequest, "invalid_router_parameters", "أدخل عنوان الراوتر وبيانات الدخول ومنفذًا صالحًا")
		return
	}
	if errors.Is(err, routerosadapter.ErrUnsupportedUserManagerVersion) {
		writeError(w, http.StatusUnprocessableEntity, "unsupported_usermanager_version", "إصدار RouterOS غير مدعوم لاستيراد User Manager")
		return
	}
	if errors.Is(err, routerosadapter.ErrInvalidUserManagerSnapshot) {
		writeError(w, http.StatusBadGateway, "invalid_usermanager_snapshot", "أعاد User Manager بيانات غير مكتملة أو غير متوافقة؛ لم تُحدّث البيانات المحلية")
		return
	}
	if err != nil {
		s.logger.Warn("User Manager read failed", "address", req.Router.Address, "error", err)
		writeError(w, http.StatusBadGateway, "usermanager_read_failed", "تعذرت قراءة User Manager. تحقق من تثبيت الحزمة وتفعيلها وصلاحيات حساب API، ثم تحقق من إصدار RouterOS.")
		return
	}

	profiles := make([]store.ImportedUserManagerProfile, 0, len(snapshot.Profiles))
	for _, profile := range snapshot.Profiles {
		profiles = append(profiles, store.ImportedUserManagerProfile{
			SourceID: profile.SourceID, Name: profile.Name, Properties: profile.Properties,
		})
	}
	limitations := make([]store.ImportedUserManagerLimitation, 0, len(snapshot.Limitations))
	for _, limitation := range snapshot.Limitations {
		limitations = append(limitations, store.ImportedUserManagerLimitation{
			SourceID: limitation.SourceID, Name: limitation.Name, Properties: limitation.Properties,
		})
	}
	relationships := make([]store.ImportedUserManagerProfileLimitation, 0, len(snapshot.ProfileLimitations))
	for _, relation := range snapshot.ProfileLimitations {
		relationships = append(relationships, store.ImportedUserManagerProfileLimitation{
			ProfileSourceID: relation.ProfileSourceID, ProfileName: relation.ProfileName,
			LimitationSourceID: relation.LimitationSourceID, LimitationName: relation.LimitationName,
			Position: relation.Position, Properties: relation.Properties,
		})
	}
	syncResult, err := s.app.DB.SyncUserManagerSnapshot(ctx, store.UserManagerSyncInput{
		Address: req.Router.Address, Port: effectiveRouterPort(req.Router),
		UseTLS: req.Router.UseTLS, RouterIdentity: snapshot.Router.Identity,
		RouterOSVersion: snapshot.Router.Version, Model: routerModel(snapshot.Router),
		Profiles: profiles, Limitations: limitations, ProfileLimitations: relationships,
	})
	if errors.Is(err, store.ErrInvalidUserManagerSnapshot) {
		writeError(w, http.StatusUnprocessableEntity, "invalid_usermanager_snapshot", "تعذر حفظ البيانات المستوردة بسبب تعارض في العلاقات أو الحقول")
		return
	}
	if err != nil {
		s.logger.Error("save User Manager snapshot failed", "address", req.Router.Address, "error", err)
		writeError(w, http.StatusInternalServerError, "usermanager_sync_failed", "تمت قراءة User Manager ولكن تعذر حفظ النسخة المحلية؛ لم تُعتمد المزامنة")
		return
	}
	profileRecords, err := s.app.DB.ListUserManagerProfiles(ctx, syncResult.RouterID)
	if err != nil {
		s.logger.Error("read saved User Manager profiles failed", "routerId", syncResult.RouterID, "error", err)
		writeError(w, http.StatusInternalServerError, "usermanager_saved_profiles_read_failed", "تم حفظ المزامنة ولكن تعذر عرض الملفات والقيود")
		return
	}
	limitationRecords, err := s.app.DB.ListUserManagerLimitations(ctx, syncResult.RouterID)
	if err != nil {
		s.logger.Error("read saved User Manager limitations failed", "routerId", syncResult.RouterID, "error", err)
		writeError(w, http.StatusInternalServerError, "usermanager_saved_limitations_read_failed", "تم حفظ المزامنة ولكن تعذر عرض القيود")
		return
	}
	writeJSON(w, http.StatusOK, userManagerOverviewResponse{
		Sync: syncResult, Router: snapshot.Router, MajorVersion: snapshot.MajorVersion,
		Profiles: profileRecords, Limitations: limitationRecords,
	})
}

func (s *Server) listSavedUserManagerProfiles(w http.ResponseWriter, r *http.Request) {
	routerID, err := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("routerId")), 10, 64)
	if err != nil || routerID < 1 {
		writeError(w, http.StatusBadRequest, "invalid_router_id", "معرّف الراوتر المستورد غير صالح")
		return
	}
	profiles, err := s.app.DB.ListUserManagerProfiles(r.Context(), routerID)
	if errors.Is(err, store.ErrImportedRouterNotFound) {
		writeError(w, http.StatusNotFound, "imported_router_not_found", "لم يتم استيراد بيانات User Manager لهذا الراوتر بعد")
		return
	}
	if err != nil {
		s.logger.Error("list saved User Manager profiles failed", "routerId", routerID, "error", err)
		writeError(w, http.StatusInternalServerError, "usermanager_profiles_read_failed", "تعذر قراءة الملفات التعريفية المستوردة")
		return
	}
	limitations, err := s.app.DB.ListUserManagerLimitations(r.Context(), routerID)
	if err != nil {
		s.logger.Error("list saved User Manager limitations failed", "routerId", routerID, "error", err)
		writeError(w, http.StatusInternalServerError, "usermanager_limitations_read_failed", "تعذر قراءة القيود المستوردة")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"profiles": profiles, "limitations": limitations})
}

func effectiveRouterPort(request routerosadapter.TestRequest) int {
	if request.Port > 0 {
		return request.Port
	}
	if request.UseTLS {
		return 8729
	}
	return 8728
}

func routerModel(info routerosadapter.RouterInfo) string {
	if strings.TrimSpace(info.Model) != "" {
		return info.Model
	}
	return info.BoardName
}

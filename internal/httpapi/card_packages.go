package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Mhmdabbasi/bt-manager/internal/store"
)

type cardPackageRequest struct {
	Name             string `json:"name"`
	ServiceSystem    string `json:"serviceSystem"`
	ServiceProfileID int64  `json:"serviceProfileId"`
	Description      string `json:"description"`
	UserPriceMinor   int64  `json:"userPriceMinor"`
	DealerPriceMinor int64  `json:"dealerPriceMinor"`
	DefaultPrefix    string `json:"defaultPrefix"`
	Active           *bool  `json:"active"`
	SortOrder        int    `json:"sortOrder"`
}

func (s *Server) listCardPackages(w http.ResponseWriter, r *http.Request) {
	includeInactive := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("includeInactive")), "true")
	serviceSystem := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("system")))
	packages, err := s.app.DB.ListCardPackages(r.Context(), includeInactive, serviceSystem)
	if errors.Is(err, store.ErrInvalidCardPackage) {
		writeError(w, http.StatusBadRequest, "invalid_card_package_filter", "نوع مصدر الباقة غير صالح؛ استخدم hotspot أو usermanager")
		return
	}
	if err != nil {
		s.logger.Error("list card packages failed", "error", err)
		writeError(w, http.StatusInternalServerError, "card_packages_read_failed", "تعذر قراءة دليل باقات الكروت")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"packages": packages})
}

func (s *Server) createCardPackage(w http.ResponseWriter, r *http.Request) {
	if !guardLocalMutation(w, r) {
		return
	}
	var req cardPackageRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "بيانات باقة الكروت غير صالحة")
		return
	}
	active := true
	if req.Active != nil {
		active = *req.Active
	}
	pkg, err := s.app.DB.CreateCardPackage(r.Context(), store.CardPackageInput{
		Name: req.Name, ServiceSystem: req.ServiceSystem, ServiceProfileID: req.ServiceProfileID,
		Description: req.Description, UserPriceMinor: req.UserPriceMinor, DealerPriceMinor: req.DealerPriceMinor,
		DefaultPrefix: req.DefaultPrefix, Active: active, SortOrder: req.SortOrder,
	})
	if s.writeCardPackageError(w, err, "create") {
		return
	}
	writeJSON(w, http.StatusCreated, pkg)
}

func (s *Server) updateCardPackage(w http.ResponseWriter, r *http.Request) {
	if !guardLocalMutation(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid_card_package_id", "رقم باقة الكروت غير صالح")
		return
	}
	var req cardPackageRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "بيانات باقة الكروت غير صالحة")
		return
	}
	active := true
	if req.Active == nil {
		existing, getErr := s.app.DB.GetCardPackage(r.Context(), id)
		if errors.Is(getErr, store.ErrCardPackageNotFound) {
			writeError(w, http.StatusNotFound, "card_package_not_found", "باقة الكروت غير موجودة")
			return
		}
		if getErr != nil {
			s.logger.Error("read card package before update failed", "packageId", id, "error", getErr)
			writeError(w, http.StatusInternalServerError, "card_package_read_failed", "تعذر قراءة باقة الكروت قبل تعديلها")
			return
		}
		active = existing.Active
	} else {
		active = *req.Active
	}
	pkg, err := s.app.DB.UpdateCardPackage(r.Context(), id, store.CardPackageInput{
		Name: req.Name, ServiceSystem: req.ServiceSystem, ServiceProfileID: req.ServiceProfileID,
		Description: req.Description, UserPriceMinor: req.UserPriceMinor, DealerPriceMinor: req.DealerPriceMinor,
		DefaultPrefix: req.DefaultPrefix, Active: active, SortOrder: req.SortOrder,
	})
	if s.writeCardPackageError(w, err, "update") {
		return
	}
	writeJSON(w, http.StatusOK, pkg)
}

func (s *Server) writeCardPackageError(w http.ResponseWriter, err error, action string) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, store.ErrInvalidCardPackage):
		writeError(w, http.StatusBadRequest, "invalid_card_package", "تحقق من اسم الباقة ونوع الخدمة والملف التعريفي والأسعار والبادئة")
	case errors.Is(err, store.ErrCardPackageServiceProfileNotFound):
		writeError(w, http.StatusNotFound, "service_profile_not_found", "الملف التعريفي المحدد غير موجود في الكتالوج المحلي؛ استورد الملفات من الراوتر أولًا")
	case errors.Is(err, store.ErrCardPackageServiceProfileUnavailable):
		writeError(w, http.StatusConflict, "service_profile_unavailable", "الملف التعريفي غير موجود حاليًا على الراوتر؛ لا يمكن تفعيل هذه الباقة")
	case errors.Is(err, store.ErrCardPackageNameExists):
		writeError(w, http.StatusConflict, "card_package_name_exists", "توجد باقة أخرى بالاسم نفسه")
	case errors.Is(err, store.ErrCardPackageNotFound):
		writeError(w, http.StatusNotFound, "card_package_not_found", "باقة الكروت غير موجودة")
	default:
		s.logger.Error("card package "+action+" failed", "error", err)
		if action == "create" {
			writeError(w, http.StatusInternalServerError, "card_package_create_failed", "تعذر حفظ باقة الكروت")
		} else {
			writeError(w, http.StatusInternalServerError, "card_package_update_failed", "تعذر تحديث باقة الكروت")
		}
	}
	return true
}

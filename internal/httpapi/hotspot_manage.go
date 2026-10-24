package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	routerosadapter "github.com/Mhmdabbasi/bt-manager/internal/routeros"
)

type hotSpotUserMutationRequest struct {
	Router   routerosadapter.TestRequest      `json:"router"`
	User     routerosadapter.HotSpotUserInput `json:"user"`
	ID       string                           `json:"id,omitempty"`
	Disabled bool                             `json:"disabled,omitempty"`
}

type hotSpotProfileMutationRequest struct {
	Router  routerosadapter.TestRequest         `json:"router"`
	Profile routerosadapter.HotSpotProfileInput `json:"profile"`
	ID      string                              `json:"id,omitempty"`
}

func (s *Server) hotSpotUserMutation(w http.ResponseWriter, r *http.Request) {
	if !guardLocalMutation(w, r) {
		return
	}
	var req hotSpotUserMutationRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "بيانات العملية غير صالحة")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	var err error
	switch r.URL.Path {
	case "/api/router/hotspot/users/create":
		err = routerosadapter.CreateHotSpotUser(ctx, req.Router, req.User)
	case "/api/router/hotspot/users/update":
		err = routerosadapter.UpdateHotSpotUser(ctx, req.Router, req.User)
	case "/api/router/hotspot/users/state":
		err = routerosadapter.SetHotSpotUserDisabled(ctx, req.Router, req.ID, req.Disabled)
	case "/api/router/hotspot/users/delete":
		err = routerosadapter.DeleteHotSpotUser(ctx, req.Router, req.ID)
	default:
		writeError(w, http.StatusNotFound, "not_found", "العملية المطلوبة غير موجودة")
		return
	}
	s.finishHotSpotMutation(w, r, err, "تعذر تنفيذ عملية المستخدم على الراوتر")
}

func (s *Server) hotSpotProfileMutation(w http.ResponseWriter, r *http.Request) {
	if !guardLocalMutation(w, r) {
		return
	}
	var req hotSpotProfileMutationRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "بيانات الملف التعريفي غير صالحة")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()

	var err error
	switch r.URL.Path {
	case "/api/router/hotspot/profiles/create":
		err = routerosadapter.CreateHotSpotProfile(ctx, req.Router, req.Profile)
	case "/api/router/hotspot/profiles/update":
		err = routerosadapter.UpdateHotSpotProfile(ctx, req.Router, req.Profile)
	case "/api/router/hotspot/profiles/delete":
		err = routerosadapter.DeleteHotSpotProfile(ctx, req.Router, req.ID)
	default:
		writeError(w, http.StatusNotFound, "not_found", "العملية المطلوبة غير موجودة")
		return
	}
	s.finishHotSpotMutation(w, r, err, "تعذر تنفيذ عملية الملف التعريفي على الراوتر")
}

func (s *Server) finishHotSpotMutation(w http.ResponseWriter, r *http.Request, err error, failureMessage string) {
	if err == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if errors.Is(err, routerosadapter.ErrInvalidRequest) {
		writeError(w, http.StatusBadRequest, "invalid_hotspot_parameters", "تحقق من الحقول المطلوبة والقيم المدخلة")
		return
	}
	s.logger.Warn("RouterOS HotSpot mutation failed", "path", r.URL.Path, "error", err)
	writeError(w, http.StatusBadGateway, "hotspot_mutation_failed", failureMessage)
}

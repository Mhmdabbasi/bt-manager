package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	routerosadapter "github.com/Mhmdabbasi/bt-manager/internal/routeros"
	"github.com/Mhmdabbasi/bt-manager/internal/store"
)

type generatePackagedVouchersRequest struct {
	Router             routerosadapter.TestRequest `json:"router"`
	PackageID          int64                       `json:"packageId"`
	TemplateID          *int64                      `json:"templateId,omitempty"`
	PointOfSaleID      *int64                      `json:"pointOfSaleId"`
	AllowWithoutTemplate bool                      `json:"allowWithoutTemplate"`
	Prefix             string                      `json:"prefix"`
	Count              int                         `json:"count"`
	Server             string                      `json:"server"`
	LimitUptime        string                      `json:"limitUptime"`
	UsernameLength     int                         `json:"usernameLength"`
	UsernameMode       string                      `json:"usernameMode"`
	PasswordMode       string                      `json:"passwordMode"`
	PasswordLength     int                         `json:"passwordLength"`
	PasswordCharset    string                      `json:"passwordCharset"`
}

// generatePackagedVouchers keeps the existing RouterOS writer, but binds every new
// batch to a locally imported package and snapshots the commercial/profile settings.
func (s *Server) generatePackagedVouchers(w http.ResponseWriter, r *http.Request) {
	if !guardLocalMutation(w, r) {
		return
	}
	var req generatePackagedVouchersRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "بيانات دفعة الباقة غير صالحة")
		return
	}
	if req.PackageID < 1 || req.Router.Address == "" || req.Router.Username == "" || req.Router.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_packaged_voucher_batch", "حدد باقة صالحة وأدخل بيانات الاتصال بالراوتر")
		return
	}
	if req.PointOfSaleID != nil && *req.PointOfSaleID < 1 {
		writeError(w, http.StatusBadRequest, "invalid_point_of_sale_id", "رقم نقطة البيع غير صالح")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	pkg, err := s.app.DB.GetCardPackage(ctx, req.PackageID)
	if errors.Is(err, store.ErrCardPackageNotFound) {
		writeError(w, http.StatusNotFound, "card_package_not_found", "باقة الكروت غير موجودة")
		return
	}
	if err != nil {
		s.logger.Error("read card package for generation failed", "packageId", req.PackageID, "error", err)
		writeError(w, http.StatusInternalServerError, "card_package_read_failed", "تعذر قراءة باقة الكروت")
		return
	}
	if !pkg.Active {
		writeError(w, http.StatusConflict, "card_package_inactive", "باقة الكروت موقوفة؛ فعّلها قبل إنشاء القسائم")
		return
	}
	if !pkg.ServiceProfilePresent {
		writeError(w, http.StatusConflict, "service_profile_unavailable", "الملف التعريفي غير موجود حاليًا على الراوتر؛ زامن الملفات قبل إنشاء القسائم")
		return
	}
	if pkg.ServiceSystem != "hotspot" {
		writeError(w, http.StatusUnprocessableEntity, "service_system_not_supported_for_generation", "توليد الباقات من User Manager غير متاح بعد؛ لا توجد أي أوامر كتابة أُرسلت إلى الراوتر")
		return
	}
	if !strings.EqualFold(strings.TrimSpace(req.Router.Address), strings.TrimSpace(pkg.RouterAddress)) ||
		effectiveRouterPort(req.Router) != pkg.RouterPort || req.Router.UseTLS != pkg.RouterUseTLS {
		writeError(w, http.StatusConflict, "package_router_mismatch", "هذه الباقة مرتبطة براوتر أو منفذ أو وضع TLS مختلف؛ اختر الراوتر الذي استوردت منه الملف التعريفي")
		return
	}

	pointOfSaleID := int64(0)
	prefix := strings.TrimSpace(req.Prefix)
	templateOverrideID := int64(0)
	if req.PointOfSaleID != nil {
		pointOfSaleID = *req.PointOfSaleID
		pos, posErr := s.app.DB.GetPointOfSale(ctx, pointOfSaleID)
		if errors.Is(posErr, store.ErrPointOfSaleNotFound) {
			writeError(w, http.StatusNotFound, "point_of_sale_not_found", "نقطة البيع غير موجودة")
			return
		}
		if posErr != nil {
			s.logger.Error("read point of sale for package generation failed", "pointOfSaleId", pointOfSaleID, "error", posErr)
			writeError(w, http.StatusInternalServerError, "point_of_sale_read_failed", "تعذر قراءة نقطة البيع")
			return
		}
		if !pos.Active {
			writeError(w, http.StatusConflict, "point_of_sale_inactive", "نقطة البيع موقوفة؛ أعد تفعيلها قبل إصدار القسائم لها")
			return
		}
		rules, rulesErr := s.app.DB.ListPointOfSalePackageRules(ctx, pointOfSaleID, true)
		if rulesErr != nil {
			s.logger.Error("read point-of-sale package permissions failed", "pointOfSaleId", pointOfSaleID, "error", rulesErr)
			writeError(w, http.StatusInternalServerError, "point_of_sale_packages_read_failed", "تعذر التحقق من الباقات المسموحة لنقطة البيع")
			return
		}
		allowed := false
		for _, rule := range rules {
			if rule.PackageID != pkg.ID {
				continue
			}
			allowed = rule.PackageActive && rule.IsEnabled
			if rule.PrefixOverride != nil && strings.TrimSpace(*rule.PrefixOverride) != "" {
				// A distributor-specific prefix is a policy override and takes precedence
				// over a one-off prefix submitted by the generation form.
				prefix = strings.TrimSpace(*rule.PrefixOverride)
			}
			if rule.TemplateIDOverride != nil {
				templateOverrideID = *rule.TemplateIDOverride
			}
			break
		}
		if !allowed {
			writeError(w, http.StatusConflict, "package_not_allowed_for_point_of_sale", "هذه الباقة غير مسموحة لنقطة البيع المحددة")
			return
		}
	}
	if prefix == "" {
		prefix = pkg.DefaultPrefix
	}

	ref, err := newVoucherBatchReference()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "reference_failed", "تعذر إنشاء رقم دفعة القسائم")
		return
	}
	input, err := routerosadapter.ValidateVoucherBatchInput(routerosadapter.VoucherBatchInput{
		Reference: ref, Prefix: prefix, Count: req.Count,
		Profile: pkg.ServiceProfileName, Server: req.Server, LimitUptime: req.LimitUptime,
		UsernameLength: req.UsernameLength, UsernameMode: req.UsernameMode,
		PasswordMode: req.PasswordMode, PasswordLength: req.PasswordLength,
		PasswordCharset: req.PasswordCharset,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_packaged_voucher_batch", "تحقق من العدد والبادئة وإعدادات رمز الدخول")
		return
	}

	var templateVersionID int64
hasTemplate := false
if req.TemplateID != nil {
	templateVersionID, err = s.app.DB.ResolveCardTemplateVersionByID(ctx, pkg.ID, pointOfSaleID, *req.TemplateID)
	if errors.Is(err, store.ErrCardTemplateVersionNotFound) {
		writeError(w, http.StatusConflict, "selected_template_unavailable", "القالب المحدد غير نشط أو لا يتوافق مع الباقة ونقطة البيع")
		return
	}
	if err != nil {
		s.logger.Error("resolve selected card template failed", "templateId", *req.TemplateID, "error", err)
		writeError(w, http.StatusInternalServerError, "card_template_resolve_failed", "تعذر تحديد نسخة قالب الطباعة")
		return
	}
	hasTemplate = true
} else {
	templateVersionID, hasTemplate, err = s.app.DB.ResolveCardTemplateVersion(ctx, pkg.ID, pointOfSaleID, templateOverrideID)
}
	if errors.Is(err, store.ErrCardTemplateVersionNotFound) {
		writeError(w, http.StatusConflict, "point_of_sale_template_version_missing", "قالب نقطة البيع المحدد لا يحتوي على نسخة محفوظة؛ حدّث القالب أو أزل التجاوز قبل التوليد")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "card_template_resolve_failed", "تعذر تحديد نسخة قالب الطباعة")
		return
	}
	if !hasTemplate && !req.AllowWithoutTemplate {
		writeError(w, http.StatusConflict, "card_template_required", "لا توجد نسخة قالب طباعة محفوظة لهذه الباقة؛ احفظ قالبًا افتراضيًا أو فعّل خيار المتابعة دون قالب محفوظ")
		return
	}

	profiles, err := s.app.DB.ListServiceProfiles(ctx, pkg.RouterID, "hotspot", true)
	if err != nil {
		s.logger.Error("read imported package profile snapshot failed", "packageId", pkg.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "service_profile_snapshot_failed", "تعذر قراءة نسخة الملف التعريفي")
		return
	}
	var profileSnapshot map[string]string
	for _, profile := range profiles {
		if profile.ID == pkg.ServiceProfileID && profile.Present {
			profileSnapshot = profile.Properties
			break
		}
	}
	if profileSnapshot == nil {
		writeError(w, http.StatusConflict, "service_profile_unavailable", "اختفى الملف التعريفي أثناء التحقق؛ زامن الملفات وأعد المحاولة")
		return
	}

	generationConfigJSON, err := json.Marshal(map[string]any{
		"count": input.Count, "prefix": input.Prefix, "server": input.Server, "limitUptime": input.LimitUptime,
		"usernameLength": input.UsernameLength, "usernameMode": input.UsernameMode,
		"passwordMode": input.PasswordMode, "passwordLength": input.PasswordLength,
		"passwordCharset": input.PasswordCharset, "allowWithoutTemplate": req.AllowWithoutTemplate,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generation_snapshot_failed", "تعذر إنشاء نسخة إعدادات التوليد")
		return
	}
	profileJSON, err := json.Marshal(profileSnapshot)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "profile_snapshot_failed", "تعذر إنشاء نسخة الملف التعريفي")
		return
	}

	userPrice, dealerPrice := pkg.UserPriceMinor, pkg.DealerPriceMinor
	batch, err := s.app.DB.BeginVoucherBatch(ctx, store.NewVoucherBatch{
		Reference: input.Reference, Prefix: input.Prefix, ProfileName: input.Profile,
		ServerName: input.Server, LimitUptime: input.LimitUptime,
		UsernameLength: input.UsernameLength, UsernameMode: input.UsernameMode,
		PasswordMode: input.PasswordMode, PasswordLength: input.PasswordLength,
		PasswordCharset: input.PasswordCharset, RequestedCount: input.Count,
		RouterID: pkg.RouterID, PackageID: pkg.ID, PointOfSaleID: pointOfSaleID,
		TemplateVersionID: templateVersionID, ServiceSystemSnapshot: pkg.ServiceSystem,
		PackageNameSnapshot: pkg.Name, UserPriceMinorSnapshot: &userPrice, DealerPriceMinorSnapshot: &dealerPrice,
		GenerationConfigJSON: string(generationConfigJSON), ServiceProfileSnapshotJSON: string(profileJSON),
		LimitationSnapshotJSON: "[]",
	})
	if err != nil {
		s.logger.Error("create packaged voucher batch failed", "packageId", pkg.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "voucher_batch_save_failed", "تعذر حفظ نسخة دفعة القسائم محليًا")
		return
	}

	result, generateErr := routerosadapter.GenerateHotSpotVouchers(ctx, req.Router, input, func(voucher routerosadapter.IssuedVoucher) error {
		_, addErr := s.app.DB.AddVoucher(ctx, batch.ID, voucher.Username, voucher.Password)
		return addErr
	})
	errorMessage := strings.TrimSpace(result.ErrorMessage)
	failedCount := result.FailedCount
	if generateErr != nil {
		if errorMessage == "" {
			errorMessage = "توقف التوليد بسبب خطأ في الاتصال أو رفض أحد الأوامر من الراوتر."
		}
		if failedCount < 1 {
			failedCount = input.Count - len(result.Created)
		}
	}
	finalizeCtx, finalizeCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer finalizeCancel()
	batch, finishErr := s.app.DB.FinishVoucherBatch(finalizeCtx, batch.ID, failedCount, errorMessage)
	if finishErr != nil {
		s.logger.Error("finish packaged voucher batch failed", "batchId", batch.ID, "error", finishErr)
		writeError(w, http.StatusInternalServerError, "voucher_batch_finalize_failed", "أُرسلت أوامر للراوتر ولكن تعذر اعتماد سجل الدفعة والمخزون محليًا")
		return
	}
	vouchers, err := s.app.DB.ListVouchersByBatch(finalizeCtx, batch.ID)
	if err != nil {
		s.logger.Error("read packaged vouchers failed", "batchId", batch.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "voucher_batch_read_failed", "تمت معالجة الدفعة لكن تعذر قراءة بطاقاتها محليًا")
		return
	}
	savedNames := make(map[string]struct{}, len(vouchers))
	for _, voucher := range vouchers {
		savedNames[voucher.Username] = struct{}{}
	}
	untracked := make([]string, 0)
	for _, created := range result.Created {
		if _, ok := savedNames[created.Username]; !ok {
			untracked = append(untracked, created.Username)
		}
	}
	if len(untracked) > 0 && errorMessage == "" {
		errorMessage = "تم إنشاء حسابات على الراوتر لم يتم حفظها محليًا؛ لا تطبع هذه القسائم قبل تسجيلها يدويًا."
	}
	s.logger.Info("packaged HotSpot voucher batch finished", "batchReference", batch.Reference,
		"packageId", pkg.ID, "pointOfSaleId", pointOfSaleID, "requested", batch.RequestedCount,
		"created", batch.CreatedCount, "failed", batch.FailedCount, "untracked", len(untracked))
	writeJSON(w, http.StatusOK, generateVouchersResponse{
		Batch: batch, Vouchers: vouchers, UntrackedUsernames: untracked, ErrorMessage: errorMessage,
	})
}

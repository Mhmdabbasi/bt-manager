package httpapi

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	routerosadapter "github.com/Mhmdabbasi/bt-manager/internal/routeros"
	"github.com/Mhmdabbasi/bt-manager/internal/store"
)

type generateVouchersRequest struct {
	Router      routerosadapter.TestRequest `json:"router"`
	Prefix      string                      `json:"prefix"`
	Count       int                         `json:"count"`
	Profile     string                      `json:"profile"`
	Server      string                      `json:"server"`
	LimitUptime string                      `json:"limitUptime"`
}

type generateVouchersResponse struct {
	Batch        store.VoucherBatch `json:"batch"`
	Vouchers     []store.Voucher    `json:"vouchers"`
	UntrackedUsernames []string    `json:"untrackedUsernames,omitempty"`
	ErrorMessage string             `json:"errorMessage,omitempty"`
}

func (s *Server) generateVouchers(w http.ResponseWriter, r *http.Request) {
	if !guardLocalMutation(w, r) {
		return
	}
	var req generateVouchersRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "بيانات دفعة القسائم غير صالحة")
		return
	}

	ref, err := newVoucherBatchReference()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "reference_failed", "تعذر إنشاء رقم دفعة القسائم")
		return
	}
	input, err := routerosadapter.ValidateVoucherBatchInput(routerosadapter.VoucherBatchInput{
		Reference: ref, Prefix: req.Prefix, Count: req.Count,
		Profile: req.Profile, Server: req.Server, LimitUptime: req.LimitUptime,
	})
	if err != nil || req.Router.Address == "" || req.Router.Username == "" || req.Router.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_voucher_batch", "تحقق من العنوان وبيانات الراوتر والبادئة والعدد والملف التعريفي")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	batch, err := s.app.DB.BeginVoucherBatch(ctx, store.NewVoucherBatch{
		Reference: input.Reference, Prefix: input.Prefix, ProfileName: input.Profile,
		ServerName: input.Server, LimitUptime: input.LimitUptime, RequestedCount: input.Count,
	})
	if err != nil {
		s.logger.Error("create voucher batch failed", "error", err)
		writeError(w, http.StatusInternalServerError, "voucher_batch_save_failed", "تعذر حفظ دفعة القسائم محليًا")
		return
	}

	result, generateErr := routerosadapter.GenerateHotSpotVouchers(ctx, req.Router, input, func(voucher routerosadapter.IssuedVoucher) error {
		_, err := s.app.DB.AddVoucher(ctx, batch.ID, voucher.Username)
		return err
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
		s.logger.Error("finish voucher batch failed", "batchId", batch.ID, "error", finishErr)
		writeError(w, http.StatusInternalServerError, "voucher_batch_finalize_failed", "أُرسلت أوامر للراوتر ولكن تعذر إكمال سجل الدفعة محليًا")
		return
	}
	vouchers, err := s.app.DB.ListVouchersByBatch(finalizeCtx, batch.ID)
	if err != nil {
		s.logger.Error("read generated vouchers failed", "batchId", batch.ID, "error", err)
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
	s.logger.Info("HotSpot voucher batch finished", "batchReference", batch.Reference,
		"requested", batch.RequestedCount, "created", batch.CreatedCount, "failed", batch.FailedCount,
		"untracked", len(untracked))
	writeJSON(w, http.StatusOK, generateVouchersResponse{
		Batch: batch, Vouchers: vouchers, UntrackedUsernames: untracked, ErrorMessage: errorMessage,
	})
}

func (s *Server) listVoucherBatches(w http.ResponseWriter, r *http.Request) {
	batches, err := s.app.DB.ListVoucherBatches(r.Context(), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "voucher_batches_read_failed", "تعذر قراءة دفعات القسائم")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"batches": batches})
}

func (s *Server) getVoucherBatch(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid_batch_id", "رقم الدفعة غير صالح")
		return
	}
	batch, err := s.app.DB.GetVoucherBatch(r.Context(), id)
	if errors.Is(err, store.ErrVoucherBatchNotFound) {
		writeError(w, http.StatusNotFound, "voucher_batch_not_found", "دفعة القسائم غير موجودة")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "voucher_batch_read_failed", "تعذر قراءة دفعة القسائم")
		return
	}
	vouchers, err := s.app.DB.ListVouchersByBatch(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "voucher_cards_read_failed", "تعذر قراءة كروت الدفعة")
		return
	}
	writeJSON(w, http.StatusOK, generateVouchersResponse{Batch: batch, Vouchers: vouchers})
}

func newVoucherBatchReference() (string, error) {
	var random [4]byte
	if _, err := crand.Read(random[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("VCH-%s-%s", time.Now().UTC().Format("20060102-150405"), hex.EncodeToString(random[:])), nil
}

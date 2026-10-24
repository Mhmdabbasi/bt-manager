package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Mhmdabbasi/bt-manager/internal/store"
)

type voucherProductRequest struct {
	Name        string `json:"name"`
	ProfileName string `json:"profileName"`
	ServerName  string `json:"serverName"`
	LimitUptime string `json:"limitUptime"`
	PriceMinor  int64  `json:"priceMinor"`
	Active      bool   `json:"active"`
}

type posSaleRequest struct {
	RequestID     string `json:"requestId"`
	ProductID    int64  `json:"productId"`
	Quantity     int    `json:"quantity"`
	PaymentMethod string `json:"paymentMethod"`
}

func (s *Server) listVoucherProducts(w http.ResponseWriter, r *http.Request) {
	includeInactive := r.URL.Query().Get("includeInactive") == "true"
	products, err := s.app.DB.ListVoucherProducts(r.Context(), includeInactive)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "voucher_products_read_failed", "تعذر قراءة باقات القسائم")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"products": products})
}

func (s *Server) createVoucherProduct(w http.ResponseWriter, r *http.Request) {
	if !guardLocalMutation(w, r) {
		return
	}
	var req voucherProductRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "بيانات الباقة غير صالحة")
		return
	}
	product, err := s.app.DB.CreateVoucherProduct(r.Context(), store.VoucherProductInput{
		Name: req.Name, ProfileName: req.ProfileName, ServerName: req.ServerName, LimitUptime: req.LimitUptime, PriceMinor: req.PriceMinor, Active: true,
	})
	if errors.Is(err, store.ErrInvalidPOSSale) {
		writeError(w, http.StatusBadRequest, "invalid_product", "تحقق من اسم الباقة والملف التعريفي والسعر")
		return
	}
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
			writeError(w, http.StatusConflict, "product_name_exists", "توجد باقة أخرى بهذا الاسم أو الملف التعريفي والخادم")
			return
		}
		s.logger.Error("create voucher product failed", "error", err)
		writeError(w, http.StatusInternalServerError, "product_create_failed", "تعذر حفظ الباقة")
		return
	}
	writeJSON(w, http.StatusCreated, product)
}

func (s *Server) updateVoucherProduct(w http.ResponseWriter, r *http.Request) {
	if !guardLocalMutation(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid_product_id", "رقم الباقة غير صالح")
		return
	}
	var req voucherProductRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "بيانات الباقة غير صالحة")
		return
	}
	product, err := s.app.DB.UpdateVoucherProduct(r.Context(), id, store.VoucherProductInput{
		Name: req.Name, ProfileName: req.ProfileName, ServerName: req.ServerName, LimitUptime: req.LimitUptime, PriceMinor: req.PriceMinor, Active: req.Active,
	})
	if errors.Is(err, store.ErrVoucherProductNotFound) {
		writeError(w, http.StatusNotFound, "product_not_found", "الباقة غير موجودة")
		return
	}
	if errors.Is(err, store.ErrInvalidPOSSale) {
		writeError(w, http.StatusBadRequest, "invalid_product", "تحقق من اسم الباقة والملف التعريفي والسعر")
		return
	}
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique constraint failed") {
			writeError(w, http.StatusConflict, "product_name_exists", "يوجد منتج آخر بهذا الاسم")
			return
		}
		s.logger.Error("update voucher product failed", "productId", id, "error", err)
		writeError(w, http.StatusInternalServerError, "product_update_failed", "تعذر تحديث الباقة")
		return
	}
	writeJSON(w, http.StatusOK, product)
}

func (s *Server) createPOSSale(w http.ResponseWriter, r *http.Request) {
	if !guardLocalMutation(w, r) {
		return
	}
	var req posSaleRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "بيانات البيع غير صالحة")
		return
	}
	reference, err := newPOSSaleReference(req.RequestID)
	if errors.Is(err, store.ErrInvalidPOSSale) {
		writeError(w, http.StatusBadRequest, "invalid_sale_request_id", "معرّف طلب البيع غير صالح")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sale_reference_failed", "تعذر إنشاء رقم البيع")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result, err := s.app.DB.CreatePOSSale(ctx, reference, req.ProductID, req.Quantity, req.PaymentMethod)
	if errors.Is(err, store.ErrInsufficientVoucherStock) {
		writeError(w, http.StatusConflict, "insufficient_stock", "المخزون المتاح لهذه الباقة لا يكفي لإتمام البيع")
		return
	}
	if errors.Is(err, store.ErrVoucherProductNotFound) {
		writeError(w, http.StatusNotFound, "product_not_found", "الباقة غير موجودة")
		return
	}
	if errors.Is(err, store.ErrInvalidPOSSale) {
		writeError(w, http.StatusBadRequest, "invalid_sale", "تحقق من الباقة والكمية وطريقة الدفع")
		return
	}
	if err != nil {
		s.logger.Error("create POS sale failed", "reference", reference, "error", err)
		writeError(w, http.StatusInternalServerError, "sale_failed", "تعذر إتمام البيع؛ لم يتم اعتماد العملية")
		return
	}
	s.logger.Info("POS sale completed", "reference", result.Sale.Reference, "quantity", len(result.Items), "totalMinor", result.Sale.TotalMinor)
	writeJSON(w, http.StatusCreated, result)
}

func (s *Server) listPOSSales(w http.ResponseWriter, r *http.Request) {
	sales, err := s.app.DB.ListPOSSales(r.Context(), 30)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "pos_sales_read_failed", "تعذر قراءة سجل المبيعات")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sales": sales})
}

func (s *Server) getPOSSale(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "invalid_sale_id", "رقم البيع غير صالح")
		return
	}
	result, err := s.app.DB.GetPOSSale(r.Context(), id)
	if errors.Is(err, store.ErrPOSSaleNotFound) {
		writeError(w, http.StatusNotFound, "sale_not_found", "عملية البيع غير موجودة")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sale_read_failed", "تعذر قراءة إيصال البيع")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func newPOSSaleReference(requestID string) (string, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		ref, err := newVoucherBatchReference()
		if err != nil {
			return "", err
		}
		return "POS-" + strings.TrimPrefix(ref, "VCH-"), nil
	}
	if len(requestID) < 8 || len(requestID) > 64 {
		return "", store.ErrInvalidPOSSale
	}
	for _, r := range requestID {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') &&
			!(r >= '0' && r <= '9') && r != '-' {
			return "", store.ErrInvalidPOSSale
		}
	}
	return "POS-" + requestID, nil
}


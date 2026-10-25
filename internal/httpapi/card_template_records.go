package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"errors"
	"net/http"
	"time"

	"github.com/Mhmdabbasi/bt-manager/internal/store"
)

type saveCardTemplateRequest struct {
	ID int64 `json:"id"`
	PackageID int64 `json:"packageId"`
	PointOfSaleID int64 `json:"pointOfSaleId"`
	Name string `json:"name"`
	IsDefault bool `json:"isDefault"`
	CardWidthMM float64 `json:"cardWidthMm"`
	CardHeightMM float64 `json:"cardHeightMm"`
	Design json.RawMessage `json:"design"`
	PageLayout json.RawMessage `json:"pageLayout"`
	BackgroundAssetID int64 `json:"backgroundAssetId"`
}

func (s *Server) listCardTemplates(w http.ResponseWriter,r *http.Request) {
	packageID:=int64(0)
	if raw:=r.URL.Query().Get("packageId");raw!="" {
		var parsed int64
		if _,err:=fmt.Sscan(raw,&parsed);err!=nil||parsed<0 { writeError(w,http.StatusBadRequest,"invalid_package_id","معرف الباقة غير صالح");return }
		packageID=parsed
	}
	ctx,cancel:=context.WithTimeout(r.Context(),5*time.Second);defer cancel()
	templates,err:=s.app.DB.ListCardTemplates(ctx,packageID)
	if err!=nil { s.logger.Error("list card templates failed","error",err);writeError(w,http.StatusInternalServerError,"card_templates_read_failed","تعذر قراءة قوالب التصميم");return }
	writeJSON(w,http.StatusOK,map[string]any{"templates":templates})
}

func (s *Server) saveCardTemplate(w http.ResponseWriter,r *http.Request) {
	if !guardLocalMutation(w,r) { return }
	r.Body=http.MaxBytesReader(w,r.Body,2<<20)
	var req saveCardTemplateRequest
	decoder:=json.NewDecoder(r.Body);decoder.DisallowUnknownFields()
	if err:=decoder.Decode(&req);err!=nil { writeError(w,http.StatusBadRequest,"invalid_card_template","بيانات القالب غير صالحة");return }
	ctx,cancel:=context.WithTimeout(r.Context(),8*time.Second);defer cancel()
	item,err:=s.app.DB.SaveCardTemplateVersion(ctx,store.SaveCardTemplateInput{
		ID:req.ID,PackageID:req.PackageID,PointOfSaleID:req.PointOfSaleID,Name:req.Name,IsDefault:req.IsDefault,
		CardWidthMM:req.CardWidthMM,CardHeightMM:req.CardHeightMM,DesignJSON:req.Design,PageLayoutJSON:req.PageLayout,
		BackgroundAssetID:req.BackgroundAssetID,
	})
	if err!=nil {
		if errors.Is(err,store.ErrCardTemplateNotFound) { writeError(w,http.StatusNotFound,"card_template_not_found","القالب المطلوب غير موجود");return }
		s.logger.Error("save card template failed","error",err)
		writeError(w,http.StatusBadRequest,"card_template_save_failed","تعذر حفظ القالب؛ تحقق من الباقة والصورة والمقاسات")
		return
	}
	writeJSON(w,http.StatusCreated,map[string]any{"template":item})
}

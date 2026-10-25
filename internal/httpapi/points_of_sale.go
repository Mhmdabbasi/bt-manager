package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Mhmdabbasi/bt-manager/internal/store"
)

type pointOfSaleRequest struct {
	Code                 string `json:"code"`
	Name                 string `json:"name"`
	Phone                string `json:"phone"`
	Address              string `json:"address"`
	Notes                string `json:"notes"`
	PackageSelectionMode string `json:"packageSelectionMode"`
	Active               *bool  `json:"active"`
}

type pointOfSalePackagesRequest struct {
	PackageSelectionMode string                              `json:"packageSelectionMode"`
	Packages             []store.PointOfSalePackageRuleInput `json:"packages"`
}

func (s *Server) listPointsOfSale(w http.ResponseWriter,r *http.Request){
	includeInactive:=strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("includeInactive")),"true")
	items,err:=s.app.DB.ListPointsOfSale(r.Context(),includeInactive)
	if err!=nil{
		s.logger.Error("list points of sale failed","error",err)
		writeError(w,http.StatusInternalServerError,"points_of_sale_read_failed","تعذرت قراءة نقاط البيع")
		return
	}
	writeJSON(w,http.StatusOK,map[string]any{"pointsOfSale":items})
}

func (s *Server) getPointOfSale(w http.ResponseWriter,r *http.Request){
	id,err:=parsePointOfSaleID(r)
	if err!=nil{writeError(w,http.StatusBadRequest,"invalid_point_of_sale_id","رقم نقطة البيع غير صالح");return}
	item,err:=s.app.DB.GetPointOfSale(r.Context(),id)
	if s.writePointOfSaleError(w,err,"read"){return}
	writeJSON(w,http.StatusOK,item)
}

func (s *Server) createPointOfSale(w http.ResponseWriter,r *http.Request){
	if !guardLocalMutation(w,r){return}
	var req pointOfSaleRequest
	if err:=decodeJSON(w,r,&req);err!=nil{writeError(w,http.StatusBadRequest,"invalid_json","بيانات نقطة البيع غير صالحة");return}
	active:=true;if req.Active!=nil{active=*req.Active}
	item,err:=s.app.DB.CreatePointOfSale(r.Context(),store.PointOfSaleInput{
		Code:req.Code,Name:req.Name,Phone:req.Phone,Address:req.Address,Notes:req.Notes,
		PackageSelectionMode:req.PackageSelectionMode,Active:active,
	})
	if s.writePointOfSaleError(w,err,"create"){return}
	writeJSON(w,http.StatusCreated,item)
}

func (s *Server) updatePointOfSale(w http.ResponseWriter,r *http.Request){
	if !guardLocalMutation(w,r){return}
	id,err:=parsePointOfSaleID(r)
	if err!=nil{writeError(w,http.StatusBadRequest,"invalid_point_of_sale_id","رقم نقطة البيع غير صالح");return}
	var req pointOfSaleRequest
	if err:=decodeJSON(w,r,&req);err!=nil{writeError(w,http.StatusBadRequest,"invalid_json","بيانات نقطة البيع غير صالحة");return}
	active:=true
	if req.Active!=nil{active=*req.Active}else{
		old,getErr:=s.app.DB.GetPointOfSale(r.Context(),id)
		if s.writePointOfSaleError(w,getErr,"read"){return}
		active=old.Active
		if strings.TrimSpace(req.PackageSelectionMode)==""{req.PackageSelectionMode=old.PackageSelectionMode}
	}
	item,err:=s.app.DB.UpdatePointOfSale(r.Context(),id,store.PointOfSaleInput{
		Code:req.Code,Name:req.Name,Phone:req.Phone,Address:req.Address,Notes:req.Notes,
		PackageSelectionMode:req.PackageSelectionMode,Active:active,
	})
	if s.writePointOfSaleError(w,err,"update"){return}
	writeJSON(w,http.StatusOK,item)
}

func (s *Server) archivePointOfSale(w http.ResponseWriter,r *http.Request){
	if !guardLocalMutation(w,r){return}
	id,err:=parsePointOfSaleID(r)
	if err!=nil{writeError(w,http.StatusBadRequest,"invalid_point_of_sale_id","رقم نقطة البيع غير صالح");return}
	item,err:=s.app.DB.ArchivePointOfSale(r.Context(),id)
	if s.writePointOfSaleError(w,err,"archive"){return}
	writeJSON(w,http.StatusOK,item)
}

func (s *Server) getPointOfSalePackages(w http.ResponseWriter,r *http.Request){
	id,err:=parsePointOfSaleID(r)
	if err!=nil{writeError(w,http.StatusBadRequest,"invalid_point_of_sale_id","رقم نقطة البيع غير صالح");return}
	pos,err:=s.app.DB.GetPointOfSale(r.Context(),id)
	if s.writePointOfSaleError(w,err,"read"){return}
	includeInactive:=strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("includeInactive")),"true")
	packages,err:=s.app.DB.ListPointOfSalePackageRules(r.Context(),id,includeInactive)
	if err!=nil{
		s.logger.Error("read point of sale package rules failed","pointOfSaleId",id,"error",err)
		writeError(w,http.StatusInternalServerError,"point_of_sale_packages_read_failed","تعذر قراءة باقات نقطة البيع")
		return
	}
	writeJSON(w,http.StatusOK,map[string]any{"pointOfSale":pos,"packages":packages})
}

func (s *Server) replacePointOfSalePackages(w http.ResponseWriter,r *http.Request){
	if !guardLocalMutation(w,r){return}
	id,err:=parsePointOfSaleID(r)
	if err!=nil{writeError(w,http.StatusBadRequest,"invalid_point_of_sale_id","رقم نقطة البيع غير صالح");return}
	var req pointOfSalePackagesRequest
	if err:=decodeJSON(w,r,&req);err!=nil{writeError(w,http.StatusBadRequest,"invalid_json","قواعد باقات نقطة البيع غير صالحة");return}
	packages,err:=s.app.DB.ReplacePointOfSalePackageRules(r.Context(),id,req.PackageSelectionMode,req.Packages)
	switch{
	case errors.Is(err,store.ErrPointOfSaleNotFound):
		writeError(w,http.StatusNotFound,"point_of_sale_not_found","نقطة البيع غير موجودة")
	case errors.Is(err,store.ErrPointOfSalePackageNotFound):
		writeError(w,http.StatusNotFound,"point_of_sale_package_not_found","إحدى الباقات المحددة غير موجودة")
	case errors.Is(err,store.ErrInvalidPointOfSalePackageRules):
		writeError(w,http.StatusBadRequest,"invalid_point_of_sale_package_rules","تحقق من وضع اختيار الباقات ومعرّفات الباقات والبادئات والقوالب المحددة")
	case err!=nil:
		s.logger.Error("replace point of sale package rules failed","pointOfSaleId",id,"error",err)
		writeError(w,http.StatusInternalServerError,"point_of_sale_packages_save_failed","تعذر حفظ باقات نقطة البيع؛ لم تُعتمد التغييرات")
		return
	}
	if err!=nil{return}
	pos,err:=s.app.DB.GetPointOfSale(r.Context(),id)
	if err!=nil{
		s.logger.Error("read point of sale after saving package rules failed","pointOfSaleId",id,"error",err)
		writeError(w,http.StatusInternalServerError,"point_of_sale_read_failed","حُفظت القواعد ولكن تعذر قراءة نقطة البيع")
		return
	}
	writeJSON(w,http.StatusOK,map[string]any{"pointOfSale":pos,"packages":packages})
}

func (s *Server) writePointOfSaleError(w http.ResponseWriter,err error,action string)bool{
	if err==nil{return false}
	switch{
	case errors.Is(err,store.ErrInvalidPointOfSale):
		writeError(w,http.StatusBadRequest,"invalid_point_of_sale","تحقق من رمز نقطة البيع واسمها وبيانات الاتصال ووضع الباقات")
	case errors.Is(err,store.ErrPointOfSaleCodeExists):
		writeError(w,http.StatusConflict,"point_of_sale_code_exists","رمز نقطة البيع مستخدم بالفعل")
	case errors.Is(err,store.ErrPointOfSaleNotFound):
		writeError(w,http.StatusNotFound,"point_of_sale_not_found","نقطة البيع غير موجودة")
	default:
		s.logger.Error("point of sale "+action+" failed","error",err)
		switch action{
		case "create":writeError(w,http.StatusInternalServerError,"point_of_sale_create_failed","تعذر إنشاء نقطة البيع")
		case "read":writeError(w,http.StatusInternalServerError,"point_of_sale_read_failed","تعذرت قراءة نقطة البيع")
		case "archive":writeError(w,http.StatusInternalServerError,"point_of_sale_archive_failed","تعذر إيقاف نقطة البيع")
		default:writeError(w,http.StatusInternalServerError,"point_of_sale_update_failed","تعذر تحديث نقطة البيع")
		}
	}
	return true
}

func parsePointOfSaleID(r *http.Request)(int64,error){
	id,err:=strconv.ParseInt(strings.TrimSpace(r.PathValue("id")),10,64)
	if err!=nil||id<1{return 0,errors.New("invalid point of sale ID")}
	return id,nil
}

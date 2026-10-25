package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Mhmdabbasi/bt-manager/internal/config"
	"github.com/Mhmdabbasi/bt-manager/internal/store"
)

const maxDesignAssetBytes = 8 << 20

type uploadDesignAssetRequest struct {
	AssetKind    string `json:"assetKind"`
	OriginalName string `json:"originalName"`
	DataBase64   string `json:"dataBase64"`
}

func (s *Server) listDesignAssets(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	assets, err := s.app.DB.ListDesignAssets(ctx)
	if err != nil {
		s.logger.Error("list design assets failed", "error", err)
		writeError(w, http.StatusInternalServerError, "design_assets_read_failed", "تعذر قراءة ملفات التصميم")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"assets": assets})
}

func (s *Server) uploadDesignAsset(w http.ResponseWriter, r *http.Request) {
	if !guardLocalMutation(w, r) { return }
	r.Body = http.MaxBytesReader(w, r.Body, 12<<20)
	var req uploadDesignAssetRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_design_asset", "بيانات الصورة غير صالحة أو أكبر من الحد المسموح")
		return
	}
	if strings.TrimSpace(req.AssetKind) == "" { req.AssetKind = "background" }
	if req.AssetKind != "background" && req.AssetKind != "image" {
		writeError(w, http.StatusBadRequest, "invalid_asset_kind", "نوع عنصر التصميم غير صالح")
		return
	}
	name := filepath.Base(strings.TrimSpace(req.OriginalName))
	if name == "." || name == string(filepath.Separator) || name == "" { name = "design-image" }
	data, err := base64.StdEncoding.DecodeString(req.DataBase64)
	if err != nil || len(data) == 0 || len(data) > maxDesignAssetBytes {
		writeError(w, http.StatusBadRequest, "invalid_asset_data", "حجم الصورة يجب أن يكون بين 1 بايت و8 ميجابايت")
		return
	}
	mimeType := http.DetectContentType(data)
	switch mimeType {
	case "image/png", "image/jpeg":
	case "image/webp":
		if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
			writeError(w, http.StatusUnsupportedMediaType, "unsupported_asset_type", "صيغة الصورة غير مدعومة")
			return
		}
	default:
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_asset_type", "يسمح فقط بصور PNG وJPG وWebP")
		return
	}
	var width, height *int
	if cfg, _, decodeErr := image.DecodeConfig(strings.NewReader(string(data))); decodeErr == nil && cfg.Width > 0 && cfg.Height > 0 {
		width, height = &cfg.Width, &cfg.Height
	}
	var keyBytes [16]byte
	if _, err := rand.Read(keyBytes[:]); err != nil {
		writeError(w, http.StatusInternalServerError, "asset_key_failed", "تعذر إنشاء معرف آمن للصورة")
		return
	}
	key := hex.EncodeToString(keyBytes[:])
	extension := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}[mimeType]
	dataDir, err := config.DefaultDataDir()
	if err != nil {
		s.logger.Error("resolve data directory for design asset failed", "error", err)
		writeError(w, http.StatusInternalServerError, "asset_storage_unavailable", "تعذر تحديد مجلد حفظ ملفات التصميم")
		return
	}
	assetDir := filepath.Join(dataDir, "design-assets")
	if err := os.MkdirAll(assetDir, 0700); err != nil {
		s.logger.Error("create design asset directory failed", "error", err)
		writeError(w, http.StatusInternalServerError, "asset_storage_unavailable", "تعذر تجهيز مجلد حفظ ملفات التصميم")
		return
	}
	filePath := filepath.Join(assetDir, key+extension)
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		s.logger.Error("write design asset failed", "error", err)
		writeError(w, http.StatusInternalServerError, "asset_save_failed", "تعذر حفظ الصورة")
		return
	}
	hash := sha256.Sum256(data)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	asset, err := s.app.DB.CreateDesignAsset(ctx, store.DesignAssetInput{
		AssetKey: key, AssetKind: req.AssetKind, OriginalName: name, MIMEType: mimeType,
		SizeBytes: int64(len(data)), SHA256: hex.EncodeToString(hash[:]), WidthPX: width, HeightPX: height,
	})
	if err != nil {
		_ = os.Remove(filePath)
		s.logger.Error("save design asset metadata failed", "error", err)
		writeError(w, http.StatusInternalServerError, "asset_metadata_save_failed", "تعذر حفظ بيانات الصورة")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"asset": asset, "url": "/api/design-assets/" + key})
}

func (s *Server) getDesignAsset(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.PathValue("key"))
	if len(key) != 32 {
		writeError(w, http.StatusBadRequest, "invalid_asset_key", "معرف الصورة غير صالح")
		return
	}
	for _, ch := range key {
		if !(ch >= '0' && ch <= '9') && !(ch >= 'a' && ch <= 'f') {
			writeError(w, http.StatusBadRequest, "invalid_asset_key", "معرف الصورة غير صالح")
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	asset, err := s.app.DB.GetDesignAssetByKey(ctx, key)
	if errors.Is(err, store.ErrDesignAssetNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "design_asset_read_failed", "تعذر قراءة بيانات الصورة")
		return
	}
	dataDir, err := config.DefaultDataDir()
	if err != nil { writeError(w, http.StatusInternalServerError, "asset_storage_unavailable", "تعذر تحديد مجلد ملفات التصميم"); return }
	extension := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/webp": ".webp"}[asset.MIMEType]
	filePath := filepath.Join(dataDir, "design-assets", asset.AssetKey+extension)
	file, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) { writeError(w, http.StatusNotFound, "asset_file_missing", "ملف الصورة غير موجود على القرص"); return }
		writeError(w, http.StatusInternalServerError, "asset_read_failed", "تعذر فتح الصورة")
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", mime.TypeByExtension(extension))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("Content-Length", fmt.Sprint(asset.SizeBytes))
	_, _ = io.Copy(w, file)
}

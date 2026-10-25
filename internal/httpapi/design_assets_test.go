package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesignAssetUploadListAndRead(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("BT_MANAGER_DATA_DIR", dataDir)
	// A valid 1x1 PNG.
	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+j3ioAAAAASUVORK5CYII=")
	if err != nil { t.Fatal(err) }
	body := `{"assetKind":"background","originalName":"test.png","dataBase64":"` + base64.StdEncoding.EncodeToString(pngBytes) + `"}`
	handler := testHandler(t)

	upload := httptest.NewRecorder()
	handler.ServeHTTP(upload, jsonRequest(http.MethodPost, "/api/design-assets", body))
	if upload.Code != http.StatusCreated { t.Fatalf("upload status %d: %s", upload.Code, upload.Body.String()) }
	var response struct { Asset struct { AssetKey string `json:"assetKey"`; MIMEType string `json:"mimeType"`; SizeBytes int64 `json:"sizeBytes"` } `json:"asset"`; URL string `json:"url"` }
	if err := json.Unmarshal(upload.Body.Bytes(), &response); err != nil { t.Fatal(err) }
	if len(response.Asset.AssetKey) != 32 || response.Asset.MIMEType != "image/png" || response.Asset.SizeBytes != int64(len(pngBytes)) {
		t.Fatalf("unexpected asset metadata: %#v", response.Asset)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "design-assets", response.Asset.AssetKey+".png")); err != nil { t.Fatalf("asset file was not persisted: %v", err) }

	list := httptest.NewRecorder()
	handler.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/design-assets", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), response.Asset.AssetKey) { t.Fatalf("list status %d: %s", list.Code, list.Body.String()) }

	read := httptest.NewRecorder()
	handler.ServeHTTP(read, httptest.NewRequest(http.MethodGet, response.URL, nil))
	if read.Code != http.StatusOK || read.Header().Get("Content-Type") != "image/png" || read.Body.Len() != len(pngBytes) {
		t.Fatalf("read status %d, content type %q, size %d", read.Code, read.Header().Get("Content-Type"), read.Body.Len())
	}
}

func TestDesignAssetRejectsNonImageAndInvalidKey(t *testing.T) {
	t.Setenv("BT_MANAGER_DATA_DIR", t.TempDir())
	handler := testHandler(t)
	bad := `{"assetKind":"background","originalName":"bad.txt","dataBase64":"` + base64.StdEncoding.EncodeToString([]byte("not an image")) + `"}`
	upload := httptest.NewRecorder()
	handler.ServeHTTP(upload, jsonRequest(http.MethodPost, "/api/design-assets", bad))
	if upload.Code != http.StatusUnsupportedMediaType { t.Fatalf("non-image status %d: %s", upload.Code, upload.Body.String()) }

	read := httptest.NewRecorder()
	handler.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/api/design-assets/invalid-key", nil))
	if read.Code != http.StatusBadRequest && read.Code != http.StatusNotFound { t.Fatalf("invalid key status %d", read.Code) }
}

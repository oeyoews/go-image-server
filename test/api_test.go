package test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"go-image-server/internal/handler"
	"go-image-server/internal/storage"

	"github.com/gin-gonic/gin"
)

func newTestServer(t *testing.T) (*gin.Engine, string) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	tmpDir, err := os.MkdirTemp("", "go-image-server-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})

	st, err := storage.NewLocalStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to init storage: %v", err)
	}

	// configPath 和 version 在测试中用不到，传空即可
	h := handler.NewUploadHandler(st, tmpDir, "", "test")

	r := gin.Default()
	apiV1 := r.Group("/api/v1")
	{
		apiV1.GET("/info", h.Info)
		apiV1.POST("/upload", h.Upload)
		apiV1.GET("/images", h.ListImages)
		apiV1.DELETE("/images", h.DeleteImage)
	}

	return r, tmpDir
}

type apiResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func TestInfoEndpoint(t *testing.T) {
	router, uploadDir := newTestServer(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp apiResponse[handler.InfoResponse]
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected response code field: %d", resp.Code)
	}

	absUploadDir, _ := filepath.Abs(uploadDir)
	if resp.Data.UploadDir != absUploadDir {
		t.Fatalf("expected upload_dir %q, got %q", absUploadDir, resp.Data.UploadDir)
	}
}

func TestUploadAndListImages(t *testing.T) {
	router, _ := newTestServer(t)

	// 构造一个简单的“图片”内容（不校验内容，只校验扩展名）
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	fileWriter, err := writer.CreateFormFile("file", "test.png")
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fileWriter.Write([]byte("PNGDATA")); err != nil {
		t.Fatalf("write fake image: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	// 上传图片
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d, body=%s", w.Code, w.Body.String())
	}

	// 列出图片（不过滤日期，直接全部）
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/images", nil)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 on list, got %d, body=%s", w.Code, w.Body.String())
	}

	var listResp apiResponse[[]handler.ImageGroup]
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("decode list response: %v", err)
	}

	if len(listResp.Data) == 0 {
		t.Fatalf("expected at least one image group, got 0")
	}
}

func TestUploadAndDeleteImage(t *testing.T) {
	router, _ := newTestServer(t)

	// 上传一张图片
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	filename := "delete-me.png"
	fileWriter, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	if _, err := fileWriter.Write([]byte("PNGDATA")); err != nil {
		t.Fatalf("write fake image: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d, body=%s", w.Code, w.Body.String())
	}

	var uploadResp apiResponse[handler.UploadResponse]
	if err := json.NewDecoder(w.Body).Decode(&uploadResp); err != nil {
		t.Fatalf("decode upload response: %v", err)
	}

	if uploadResp.Data.Path == "" {
		t.Fatalf("expected non-empty path in upload response")
	}

	// 删除刚上传的图片
	deleteURL := "/api/v1/images?path=" + uploadResp.Data.Path
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, deleteURL, nil)

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 on delete, got %d, body=%s", w.Code, w.Body.String())
	}

	var delResp apiResponse[handler.DeleteResult]
	if err := json.NewDecoder(w.Body).Decode(&delResp); err != nil {
		t.Fatalf("decode delete response: %v", err)
	}

	if !delResp.Data.OK {
		t.Fatalf("expected delete ok=true, got false")
	}

	// 再次删除应返回 404
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, deleteURL, nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 on second delete, got %d", w.Code)
	}
}


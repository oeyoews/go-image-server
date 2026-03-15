package handler

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"go-image-server/internal/storage"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type UploadHandler struct {
	storage    *storage.LocalStorage
	uploadDir  string
	configPath string
	version    string
}

func NewUploadHandler(s *storage.LocalStorage, uploadDir, configPath, version string) *UploadHandler {
	return &UploadHandler{storage: s, uploadDir: uploadDir, configPath: configPath, version: version}
}

// InfoResponse 服务信息响应
type InfoResponse struct {
	Version    string `json:"version"`
	UploadDir  string `json:"upload_dir"`
	ConfigFile string `json:"config_file"`
}

// Info godoc
// @Summary      服务信息
// @Description  返回服务端存储目录与配置文件路径，供前端展示
// @Tags         info
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /info [get]
func (h *UploadHandler) Info(c *gin.Context) {
	uploadDir := h.uploadDir
	if abs, err := filepath.Abs(uploadDir); err == nil {
		uploadDir = abs
	}
	respOK(c, InfoResponse{
		Version:    h.version,
		UploadDir:  uploadDir,
		ConfigFile: h.configPath,
	})
}

type UploadResponse struct {
	URL  string `json:"url"`
	Path string `json:"path"`
}

type ImageFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

type ImageGroup struct {
	Date  string      `json:"date"`
	Files []ImageFile `json:"files"`
}

// DeleteResult 删除接口成功时 data 字段
type DeleteResult struct {
	OK bool `json:"ok"`
}

// Upload godoc
// @Summary      上传图片
// @Description  上传一张图片并返回访问地址
// @Tags         images
// @Accept       multipart/form-data
// @Produce      json
// @Param        file      formData  file  true   "图片文件"
// @Param        filename  formData  string false "保存时的文件名（可选，上传前改名）"
// @Success      201  {object}  APIResponse
// @Failure      400  {object}  APIError
// @Failure      500  {object}  APIError
// @Router       /upload [post]
func (h *UploadHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		respBadRequest(c, "missing file")
		return
	}

	if file.Size > storage.MaxFileSize {
		respBadRequest(c, "file too large")
		return
	}

	f, err := file.Open()
	if err != nil {
		respServerError(c, "failed to read file")
		return
	}
	defer f.Close()

	// 允许通过 form 字段 filename 指定保存时的文件名（上传前改名）
	saveName := c.PostForm("filename")
	if saveName == "" {
		saveName = file.Filename
	}
	relPath, err := h.storage.Save(f, saveName)
	if err != nil {
		respServerError(c, err.Error())
		return
	}

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	url := scheme + "://" + c.Request.Host + "/files/" + relPath
	humanSize := formatSize(file.Size)
	log.WithFields(log.Fields{
		"path":       relPath,
		"url":        url,
		"size_bytes": file.Size,
		"size":       humanSize,
	}).Info("Image uploaded")

	respCreated(c, UploadResponse{
		URL:  url,
		Path: relPath,
	})
}

// ListImages 按日期返回图片列表，支持 ?date=YYYY-MM-DD 过滤。
// @Summary      图片列表
// @Description  按日期分组返回图片列表，可通过 date=YYYY-MM-DD 过滤
// @Tags         images
// @Produce      json
// @Param        date  query  string  false  "日期(YYYY-MM-DD)"
// @Success      200  {object}  APIResponse
// @Failure      500  {object}  APIError
// @Router       /images [get]
func (h *UploadHandler) ListImages(c *gin.Context) {
	date := c.Query("date")

	groups, err := h.storage.ListByDate(date)
	if err != nil {
		respServerError(c, err.Error())
		return
	}

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}

	host := c.Request.Host
	var result []ImageGroup
	for day, files := range groups {
		group := ImageGroup{Date: day}
		for _, f := range files {
			url := scheme + "://" + host + "/files/" + f.Path
			group.Files = append(group.Files, ImageFile{
				Name: f.Name,
				Path: f.Path,
				URL:  url,
				Size: f.Size,
			})
		}
		result = append(result, group)
	}

	// 简单按日期倒序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date > result[j].Date
	})

	respOK(c, result)
}

func formatSize(size int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// DeleteImage 删除指定路径的图片，使用 query 参数 path=YYYY-MM-DD/xxx.png。
// @Summary      删除图片
// @Description  通过相对路径删除图片，path 形如 2026-03-14/xxx.png
// @Tags         images
// @Produce      json
// @Param        path  query  string  true  "相对路径(YYYY-MM-DD/xxx.png)"
// @Success      200  {object}  APIResponse
// @Failure      400  {object}  APIError
// @Failure      404  {object}  APIError
// @Failure      500  {object}  APIError
// @Router       /images [delete]
func (h *UploadHandler) DeleteImage(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		respBadRequest(c, "missing path")
		return
	}

	err := h.storage.Delete(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			respNotFound(c, "not found")
			return
		}
		respServerError(c, err.Error())
		return
	}

	log.WithField("path", path).Info("Image deleted")
	respOK(c, DeleteResult{OK: true})
}

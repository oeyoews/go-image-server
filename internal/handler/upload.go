package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"

	"go-image-server/internal/storage"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type UploadHandler struct {
	storage *storage.LocalStorage
}

func NewUploadHandler(s *storage.LocalStorage) *UploadHandler {
	return &UploadHandler{storage: s}
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

// Upload godoc
// @Summary      上传图片
// @Description  上传一张图片并返回访问地址
// @Tags         images
// @Accept       multipart/form-data
// @Produce      json
// @Param        file      formData  file  true   "图片文件"
// @Param        filename  formData  string false "保存时的文件名（可选，上传前改名）"
// @Success      200   {object}  UploadResponse
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /upload [post]
func (h *UploadHandler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
		return
	}

	if file.Size > storage.MaxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file too large"})
		return
	}

	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	c.JSON(http.StatusOK, UploadResponse{
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
// @Success      200   {array}  ImageGroup
// @Failure      500   {object}  map[string]string
// @Router       /images [get]
func (h *UploadHandler) ListImages(c *gin.Context) {
	date := c.Query("date")

	groups, err := h.storage.ListByDate(date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

	c.JSON(http.StatusOK, result)
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
// @Success      200   {object}  map[string]bool
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Router       /images [delete]
func (h *UploadHandler) DeleteImage(c *gin.Context) {
	path := c.Query("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing path"})
		return
	}

	err := h.storage.Delete(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.WithField("path", path).Info("Image deleted")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

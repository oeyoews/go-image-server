package upload

import (
	"go-image-server/internal/storage"
	"go-image-server/internal/utils"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

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
func (h *Handler) Upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		respBadRequest(c, "missing file")
		return
	}

	// 使用 storage 层统一的文件大小限制，避免接口层出现硬编码的 magic number
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

	// 始终返回 http 协议的访问地址，保持与前端协议约定一致
	url := "http://" + c.Request.Host + "/files/" + relPath
	humanSize := utils.FormatSize(file.Size)
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


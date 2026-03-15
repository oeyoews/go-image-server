package upload

import (
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// Info godoc
// @Summary      服务信息
// @Description  返回服务端存储目录与配置文件路径，供前端展示
// @Tags         info
// @Produce      json
// @Success      200  {object}  APIResponse
// @Router       /info [get]
func (h *Handler) Info(c *gin.Context) {
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


package upload

import (
	"errors"
	"os"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

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
func (h *Handler) DeleteImage(c *gin.Context) {
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


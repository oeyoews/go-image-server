package files

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"go-image-server/internal/storage"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	storage *storage.Manager
}

func NewHandler(m *storage.Manager) *Handler {
	return &Handler{storage: m}
}

func (h *Handler) Get(c *gin.Context) {
	st := h.storage.Get()
	key := strings.TrimPrefix(c.Param("path"), "/")
	if key == "" {
		c.Status(http.StatusNotFound)
		return
	}

	reader, contentType, size, err := st.Open(context.Background(), key)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.Status(http.StatusNotFound)
			return
		}
		c.String(http.StatusInternalServerError, err.Error())
		return
	}
	defer reader.Close()

	if contentType != "" {
		c.Header("Content-Type", contentType)
	}
	if size >= 0 {
		c.Header("Content-Length", strconv.FormatInt(size, 10))
	}
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, reader)
}

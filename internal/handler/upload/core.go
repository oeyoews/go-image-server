package upload

import (
	"go-image-server/internal/storage"
)

// Handler 聚合存储实现及与上传相关的上下文信息。
// 每个字段都与对外 API 有关，例如上传目录与配置文件路径用于 Info 接口展示。
type Handler struct {
	storage    *storage.LocalStorage
	uploadDir  string
	configPath string
	version    string
}

// NewHandler 构造一个上传处理器实例，供 HTTP 路由注册时使用。
func NewHandler(s *storage.LocalStorage, uploadDir, configPath, version string) *Handler {
	return &Handler{storage: s, uploadDir: uploadDir, configPath: configPath, version: version}
}

// InfoResponse 服务信息响应
type InfoResponse struct {
	Version    string `json:"version"`
	UploadDir  string `json:"upload_dir"`
	ConfigFile string `json:"config_file"`
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


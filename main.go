package main

// @title           Go Image Server API
// @version         1.0
// @description     Local image upload and management service.
// @BasePath        /

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go-image-server/internal/handler"
	"go-image-server/internal/storage"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/mitchellh/go-homedir"
)

type Config struct {
	Port      string `json:"port"`
	UploadDir string `json:"upload_dir"`
}

func defaultConfig() Config {
	return Config{
		Port:      "8080",
		UploadDir: "",
	}
}

func writeDefaultConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(defaultConfig())
}

func loadConfig() (Config, string) {
	path := os.Getenv("CONFIG_FILE")
	if path == "" {
		if cfgDir, err := os.UserConfigDir(); err == nil && cfgDir != "" {
			path = filepath.Join(cfgDir, "go-image-server", "config.json")
		} else {
			path = "config.json"
		}
	}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := writeDefaultConfig(path); err != nil {
				log.WithError(err).Errorf("failed to write default config %s", path)
				return defaultConfig(), path
			}
			return defaultConfig(), path
		}
		return defaultConfig(), path
	}
	defer f.Close()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		log.WithError(err).Errorf("failed to parse config file %s", path)
		return defaultConfig(), path
	}
	return cfg, path
}

func resolveUploadDir(cfg Config) string {
	if dir := os.Getenv("UPLOAD_DIR"); dir != "" {
		return expandHome(dir)
	}
	if cfg.UploadDir != "" {
		return expandHome(cfg.UploadDir)
	}

	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
		return filepath.Join(cacheDir, "go-image-server", "uploads")
	}

	return "./uploads"
}

func expandHome(path string) string {
	if path == "" {
		return path
	}
	expanded, err := homedir.Expand(path)
	if err != nil {
		return path
	}
	return expanded
}

func setupLogger() {
	// 日志格式：带时间、颜色、高亮级别
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		ForceColors:     true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	level := strings.ToLower(os.Getenv("LOG_LEVEL"))
	switch level {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info", "":
		log.SetLevel(log.InfoLevel)
	case "warn", "warning":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}
}

func main() {
	setupLogger()
	gin.SetMode(gin.ReleaseMode) // 关闭 [GIN-debug] 路由注册日志

	cfg, cfgPath := loadConfig()
	uploadDir := resolveUploadDir(cfg)

	log.WithField("path", cfgPath).Info("Config file path")
	log.WithFields(log.Fields{
		"port":       cfg.Port,
		"upload_dir": cfg.UploadDir,
	}).Info("Config loaded")
	log.WithField("upload_dir", uploadDir).Info("Resolved upload dir")

	st, err := storage.NewLocalStorage(uploadDir)
	if err != nil {
		log.WithError(err).Fatal("Failed to init storage")
	}

	h := handler.NewUploadHandler(st, uploadDir, cfgPath)

	r := gin.Default()
	r.Use(cors.Default()) // 允许跨域请求
	// 直接以路径访问图片，例如 /files/2026-03-14/xxx.png
	r.StaticFS("/files", gin.Dir(uploadDir, false))
	r.GET("/info", h.Info)
	r.POST("/upload", h.Upload)
	r.GET("/images", h.ListImages)
	r.DELETE("/images", h.DeleteImage)

	port := os.Getenv("PORT")
	if port == "" {
		if cfg.Port != "" {
			port = cfg.Port
		} else {
			port = "8080"
		}
	}

	basePort, err := strconv.Atoi(port)
	if err != nil || basePort <= 0 {
		basePort = 8080
	}

	const maxTries = 10
	for i := 0; i < maxTries; i++ {
		p := basePort + i
		addr := fmt.Sprintf(":%d", p)
		log.WithField("port", p).Info("Server starting")
		if err := r.Run(addr); err != nil {
			msg := err.Error()
			if strings.Contains(msg, "address already in use") || strings.Contains(msg, "Only one usage of each socket address") {
				log.WithField("port", p).Warn("Port in use, trying next")
				continue
			}
			log.WithError(err).Fatal("Server exited")
		}
		break
	}
}

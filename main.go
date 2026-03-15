package main

// @title           Go Image Server API
// @version         1.0
// @description     Local image upload and management service.
// @BasePath        /api/v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"go-image-server/internal/handler"
	"go-image-server/internal/storage"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
)

// Version 版本号，构建时可覆盖: go build -ldflags "-X main.Version=1.0.0"
var Version = "1.0.0"

type Config struct {
	Port        string `json:"port"`
	UploadDir   string `json:"upload_dir"`
	OpenBrowser bool   `json:"open_browser"`
}

func getDefaultUploadDir() string {
	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
		return filepath.Join(cacheDir, "go-image-server", "uploads")
	}
	return "./uploads"
}

func defaultConfig() Config {
	return Config{
		Port:        "48083",
		UploadDir:   getDefaultUploadDir(),
		OpenBrowser: true,
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
		if errors.Is(err, io.EOF) {
			log.WithField("path", path).Info("Config file is empty, re-initializing")
			if err := writeDefaultConfig(path); err != nil {
				log.WithError(err).Errorf("failed to write default config %s", path)
			}
		} else {
			log.WithError(err).Errorf("failed to parse config file %s", path)
		}
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

func openBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		// 使用系统默认浏览器打开
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // linux, *bsd 等
		cmd = exec.Command("xdg-open", url)
	}

	if err := cmd.Start(); err != nil {
		log.WithError(err).WithField("url", url).Warn("Failed to open browser")
	}
}

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "-version" || arg == "--version" || arg == "-v" {
			fmt.Println(Version)
			os.Exit(0)
		}
	}

	setupLogger()

	mode := strings.ToLower(os.Getenv("GIN_MODE"))
	if mode == "" {
		mode = strings.ToLower(os.Getenv("APP_ENV"))
	}

	isDev := mode == "debug" || mode == "development" || mode == "dev"

	if isDev {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode) // 关闭 [GIN-debug] 路由注册日志
	}

	cfg, cfgPath := loadConfig()
	uploadDir := resolveUploadDir(cfg)

	log.Infof("Config file path: %s", cfgPath)
	log.Infof("Config loaded: port=%s upload_dir=%s", cfg.Port, cfg.UploadDir)
	log.Infof("Resolved upload dir: %s", uploadDir)

	st, err := storage.NewLocalStorage(uploadDir)
	if err != nil {
		log.WithError(err).Fatal("Failed to init storage")
	}

	h := handler.NewUploadHandler(st, uploadDir, cfgPath, Version)

	r := gin.Default()
	r.Use(cors.Default()) // 允许跨域请求
	// 直接以路径访问图片，例如 /files/2026-03-14/xxx.png
	r.StaticFS("/files", gin.Dir(uploadDir, false))
	apiV1 := r.Group("/api/v1")
	{
		apiV1.GET("/info", h.Info)
		apiV1.POST("/upload", h.Upload)
		apiV1.GET("/images", h.ListImages)
		apiV1.DELETE("/images", h.DeleteImage)
	}

	// 开发模式下启用 Swagger UI: /swagger/index.html（仅 dev 构建有效）
	registerSwagger(r, isDev)

	// 原版上传页：Vue3 + Tailwind CDN，同 neotw-image-upload 插件功能
	r.GET("/", func(c *gin.Context) { c.File("static/index.html") })

	port := cfg.Port
	if port == "" {
		port = "48083"
	}

	basePort, err := strconv.Atoi(port)
	if err != nil || basePort <= 0 {
		basePort = 48083
	}

	const maxTries = 10
	for i := 0; i < maxTries; i++ {
		p := basePort + i
		addr := fmt.Sprintf(":%d", p)

		ln, err := net.Listen("tcp", addr)
		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "address already in use") || strings.Contains(msg, "Only one usage of each socket address") {
				log.WithField("port", p).Warn("Port in use, trying next")
				continue
			}
			log.WithError(err).Fatal("Failed to listen")
		}

		uploadPageURL := fmt.Sprintf("http://localhost:%d/", p)
		log.Infof("Server starting on port %d", p)
		log.Infof("Web upload page: %s", uploadPageURL)

		if cfg.OpenBrowser {
			go openBrowser(uploadPageURL)
		}

		if err := http.Serve(ln, r); err != nil {
			log.WithError(err).Fatal("Server exited")
		}
		break
	}
}

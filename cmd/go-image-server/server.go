package main

// 负责 HTTP 服务器的初始化、路由注册与端口监听逻辑。
// 将与启动流程相关的代码与 main.go 入口解耦，便于维护与测试。

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"

	"go-image-server/internal/handler/files"
	"go-image-server/internal/handler/upload"
	"go-image-server/internal/storage"
	"go-image-server/internal/utils"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// setupLogger 根据环境变量 LOG_LEVEL 配置全局 logrus 日志格式与级别。
func setupLogger() {
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

// run 负责完整的服务启动流程：
// - 读取运行模式与配置
// - 初始化存储与上传处理器
// - 注册 Gin 路由与 Swagger
// - 尝试多个端口并启动 HTTP 服务器
func run() {
	setupLogger()

	mode := strings.ToLower(os.Getenv("GIN_MODE"))
	if mode == "" {
		// 未显式设置 GIN_MODE 时，允许通过 APP_ENV 控制定制环境（dev / production）
		mode = strings.ToLower(os.Getenv("APP_ENV"))
	}

	// 约定几个“开发模式”的别名，便于通过环境变量切换日志与路由输出
	devModes := []string{"debug", "development", "dev"}
	isDev := slices.Contains(devModes, mode)
	log.Infof("Mode: %s, isDev: %v", mode, isDev)

	gin.SetMode(gin.ReleaseMode)
	if isDev {
		gin.SetMode(gin.DebugMode)
	}

	cfg, cfgPath := loadConfig()
	storageCfg := resolveStorageConfig(cfg)
	uploadDir := resolveUploadDir(cfg)
	if storageCfg.Type == "local" {
		uploadDir = storageCfg.Local.BaseDir
	}

	log.Infof("Config file path: %s", cfgPath)
	log.Infof("Config loaded: port=%s upload_dir=%s", cfg.Port, uploadDir)

	st, err := storage.New(storageCfg)
	if err != nil {
		log.WithError(err).Fatal("Failed to init storage")
	}

	sm := storage.NewManager(st, storageCfg)
	h := upload.NewHandler(sm, cfgPath, Version, cfg.PreviewImageList)
	fh := files.NewHandler(sm)

	r := gin.Default()
	r.Use(cors.Default())
	r.GET("/files/*path", fh.Get)
	apiV1 := r.Group("/api/v1")
	{
		apiV1.GET("/info", h.Info)
		apiV1.POST("/upload", h.Upload)
		apiV1.GET("/images", h.ListImages)
		apiV1.DELETE("/images", h.DeleteImage)
		apiV1.GET("/settings/storage", func(c *gin.Context) {
			c.JSON(http.StatusOK, upload.APIResponse{
				Code:    http.StatusOK,
				Message: "success",
				Data:    sm.GetConfig(),
			})
		})
		apiV1.PUT("/settings/storage", func(c *gin.Context) {
			var sc storage.DriverConfig
			if err := c.ShouldBindJSON(&sc); err != nil {
				c.JSON(http.StatusBadRequest, upload.APIResponse{Code: http.StatusBadRequest, Message: err.Error(), Data: nil})
				return
			}

			resolved := resolveStorageConfig(Config{Storage: sc, UploadDir: cfg.UploadDir})
			cfg.Storage = sc

			if err := sm.Set(resolved); err != nil {
				c.JSON(http.StatusBadRequest, upload.APIResponse{Code: http.StatusBadRequest, Message: err.Error(), Data: nil})
				return
			}
			if err := writeConfig(cfgPath, cfg); err != nil {
				c.JSON(http.StatusInternalServerError, upload.APIResponse{Code: http.StatusInternalServerError, Message: err.Error(), Data: nil})
				return
			}
			c.JSON(http.StatusOK, upload.APIResponse{Code: http.StatusOK, Message: "success", Data: sc})
		})

		// UI 相关设置，目前仅包含图片列表是否预览缩略图
		apiV1.PUT("/settings/preview", func(c *gin.Context) {
			var body struct {
				PreviewImageList *bool `json:"preview_image_list"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, upload.APIResponse{Code: http.StatusBadRequest, Message: err.Error(), Data: nil})
				return
			}
			if body.PreviewImageList == nil {
				c.JSON(http.StatusBadRequest, upload.APIResponse{Code: http.StatusBadRequest, Message: "preview_image_list is required", Data: nil})
				return
			}
			cfg.PreviewImageList = *body.PreviewImageList
			// 同步更新到处理器实例，保证 /info 接口与运行时行为一致
			h.SetPreviewImageList(cfg.PreviewImageList)
			if err := writeConfig(cfgPath, cfg); err != nil {
				c.JSON(http.StatusInternalServerError, upload.APIResponse{Code: http.StatusInternalServerError, Message: err.Error(), Data: nil})
				return
			}
			c.JSON(http.StatusOK, upload.APIResponse{
				Code:    http.StatusOK,
				Message: "success",
				Data: map[string]any{
					"preview_image_list": cfg.PreviewImageList,
				},
			})
		})
	}

	registerSwagger(r, isDev)

	r.GET("/", func(c *gin.Context) { c.File("web/index.html") })

	// 允许通过配置指定基础端口，留空则回退到默认端口
	port := cfg.Port
	if port == "" {
		port = "48083"
	}

	// 配置既可以是字符串端口，也可以是非法值，此处做一次兜底
	basePort, err := strconv.Atoi(port)
	if err != nil || basePort <= 0 {
		basePort = 48083
	}

	// 为了避免端口占用导致启动失败，这里从基础端口开始，向后尝试最多 10 次
	const maxTries = 10
	for i := 0; i < maxTries; i++ {
		p := basePort + i
		addr := fmt.Sprintf(":%d", p)

		// 手动 Listen 端口，便于在打开浏览器前获知实际监听的端口号
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			msg := err.Error()
			// windows 与类 Unix 系统的端口占用错误信息略有差异，这里做简单字符串匹配
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
			go utils.OpenBrowser(uploadPageURL)
		}

		if err := http.Serve(ln, r); err != nil {
			log.WithError(err).Fatal("Server exited")
		}
		break
	}
}

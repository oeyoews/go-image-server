package main

// 负责读取与解析配置文件、提供默认配置以及上传目录的解析逻辑。
// 将 I/O 与配置细节集中在这里，避免散落在启动流程中。

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"go-image-server/internal/storage"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
)

// Config 描述服务运行所需的基础配置。
// 这些字段既可以通过配置文件设置，也可以部分被环境变量覆盖。
type Config struct {
	Port             string               `json:"port"`
	UploadDir        string               `json:"upload_dir"`
	OpenBrowser      bool                 `json:"open_browser"`
	PreviewImageList bool                 `json:"preview_image_list"`
	Storage          storage.DriverConfig `json:"storage"`
}

// getDefaultUploadDir 返回平台相关的默认上传目录。
// 优先使用用户缓存目录，找不到时退回到本地相对路径。
func getDefaultUploadDir() string {
	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
		return filepath.Join(cacheDir, "go-image-server", "uploads")
	}
	return "./uploads"
}

// defaultConfig 返回一份带有合理默认值的配置，用于首次运行或配置读取失败时兜底。
func defaultConfig() Config {
	return Config{
		Port:             "48083",
		UploadDir:        getDefaultUploadDir(),
		OpenBrowser:      true,
		PreviewImageList: false,
		Storage: storage.DriverConfig{
			Type: "local",
			Local: storage.LocalConfig{
				BaseDir: getDefaultUploadDir(),
			},
		},
	}
}

// writeDefaultConfig 在指定路径写入一份默认配置文件。
// 若上层目录不存在会自动创建，写入失败会返回错误给调用方。
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

// loadConfig 从配置文件加载配置，如果文件不存在或解析失败则回退到默认配置。
// 返回值为配置对象以及实际使用的配置文件路径，便于在接口中展示。
func loadConfig() (Config, string) {
	path := os.Getenv("CONFIG_FILE")
	if path == "" {
		// 未指定 CONFIG_FILE 时，优先使用平台推荐的配置目录，其次退回到当前工作目录
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

// resolveUploadDir 结合环境变量 UPLOAD_DIR 与配置文件中的 UploadDir 计算最终上传目录。
// 同时会展开 ~ 为用户主目录，保证返回的是可用路径。
func resolveUploadDir(cfg Config) string {
	if dir := os.Getenv("UPLOAD_DIR"); dir != "" {
		// 环境变量优先于配置文件，便于在容器或 CI 场景下覆盖默认行为
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

func resolveStorageConfig(cfg Config) storage.DriverConfig {
	sc := cfg.Storage
	if sc.Type == "" {
		sc.Type = "local"
	}

	if dir := os.Getenv("UPLOAD_DIR"); dir != "" {
		// 环境变量优先，其值同样支持 ~ 展开
		sc.Local.BaseDir = expandHome(dir)
	} else if sc.Local.BaseDir == "" {
		// 未显式配置本地存储目录时，退回到 UploadDir 或默认目录
		if cfg.UploadDir != "" {
			sc.Local.BaseDir = expandHome(cfg.UploadDir)
		} else {
			sc.Local.BaseDir = getDefaultUploadDir()
		}
	} else {
		// 显式配置的本地目录也需要支持 ~ 展开
		sc.Local.BaseDir = expandHome(sc.Local.BaseDir)
	}

	if sc.GitHub.Token == "" && sc.GitHub.TokenEnv != "" {
		sc.GitHub.Token = os.Getenv(sc.GitHub.TokenEnv)
	}
	if sc.GitLab.Token == "" && sc.GitLab.TokenEnv != "" {
		sc.GitLab.Token = os.Getenv(sc.GitLab.TokenEnv)
	}
	return sc
}

func writeConfig(path string, cfg Config) error {
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
	return enc.Encode(cfg)
}

// expandHome 支持将以 ~ 开头的路径展开为当前用户的 home 目录。
// 如果展开失败则返回原始字符串以避免中断主流程。
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

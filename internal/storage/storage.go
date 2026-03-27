package storage

import (
	"context"
	"fmt"
	"io"
	"sync"
)

const (
	MaxFileSize = 5 << 20 // 5MB
)

type StoredObject struct {
	Key       string
	DirectURL string
	Size      int64
}

type FileInfo struct {
	Name      string
	Path      string
	Size      int64
	DirectURL string
}

type Storage interface {
	Save(ctx context.Context, r io.Reader, originalFilename string) (StoredObject, error)
	Delete(ctx context.Context, key string) error
	ListByDate(ctx context.Context, date string) (map[string][]FileInfo, error)
	Open(ctx context.Context, key string) (io.ReadCloser, string, int64, error)
	ResolveDirectURL(key string) string
}

type DriverConfig struct {
	Type   string       `json:"type"`
	Local  LocalConfig  `json:"local"`
	GitHub GitHubConfig `json:"github"`
	GitLab GitLabConfig `json:"gitlab"`
}

type LocalConfig struct {
	BaseDir string `json:"base_dir"`
}

type GitHubConfig struct {
	Owner     string `json:"owner"`
	Repo      string `json:"repo"`
	Branch    string `json:"branch"`
	PathPrefix string `json:"path_prefix"`
	Token     string `json:"token,omitempty"`
	TokenEnv  string `json:"token_env"`
}

type GitLabConfig struct {
	BaseURL       string `json:"base_url"`
	Project       string `json:"project"`
	Branch        string `json:"branch"`
	PathPrefix    string `json:"path_prefix"`
	Token         string `json:"token,omitempty"`
	TokenEnv      string `json:"token_env"`
}

type Factory func(cfg DriverConfig) (Storage, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

func Register(name string, factory Factory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = factory
}

func New(cfg DriverConfig) (Storage, error) {
	if cfg.Type == "" {
		cfg.Type = "local"
	}

	registryMu.RLock()
	factory, ok := registry[cfg.Type]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown storage type: %s", cfg.Type)
	}
	return factory(cfg)
}

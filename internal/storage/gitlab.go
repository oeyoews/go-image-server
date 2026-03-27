package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type GitLabStorage struct {
	baseURL    string
	project    string
	branch     string
	pathPrefix string
	token      string
	client     *http.Client
}

func init() {
	Register("gitlab", func(cfg DriverConfig) (Storage, error) {
		return NewGitLabStorage(cfg.GitLab)
	})
}

func NewGitLabStorage(cfg GitLabConfig) (*GitLabStorage, error) {
	if cfg.Project == "" {
		return nil, fmt.Errorf("gitlab project is required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://gitlab.com"
	}
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	token := cfg.Token
	if token == "" && cfg.TokenEnv != "" {
		token = strings.TrimSpace(os.Getenv(cfg.TokenEnv))
	}
	if token == "" {
		return nil, fmt.Errorf("gitlab token is required")
	}

	return &GitLabStorage{
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		project:    cfg.Project,
		branch:     cfg.Branch,
		pathPrefix: strings.Trim(cfg.PathPrefix, "/"),
		token:      token,
		client:     &http.Client{Timeout: 20 * time.Second},
	}, nil
}

func (s *GitLabStorage) Save(ctx context.Context, r io.Reader, originalFilename string) (StoredObject, error) {
	filename, _, err := ensureAllowedFilename(originalFilename)
	if err != nil {
		return StoredObject{}, err
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxFileSize+1))
	if err != nil {
		return StoredObject{}, err
	}
	if int64(len(data)) > MaxFileSize {
		return StoredObject{}, fmt.Errorf("file too large")
	}

	key := path.Join(todayDir(), withTimePrefix(filename))
	filePath := s.repoPath(key)
	endpoint := s.fileEndpoint(filePath)
	reqBody := map[string]string{
		"branch":         s.branch,
		"content":        string(data),
		"encoding":       "base64",
		"commit_message": "upload image: " + key,
	}
	reqBody["content"] = base64.StdEncoding.EncodeToString(data)
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return StoredObject{}, err
	}
	s.applyHeaders(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return StoredObject{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return StoredObject{}, fmt.Errorf("gitlab upload failed: %s", strings.TrimSpace(string(raw)))
	}

	return StoredObject{
		Key:       key,
		DirectURL: s.ResolveDirectURL(key),
		Size:      int64(len(data)),
	}, nil
}

func (s *GitLabStorage) Delete(ctx context.Context, key string) error {
	filePath := s.repoPath(key)
	endpoint := s.fileEndpoint(filePath) + "?branch=" + url.QueryEscape(s.branch) + "&commit_message=" + url.QueryEscape("delete image: "+key)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	s.applyHeaders(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return os.ErrNotExist
	}
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gitlab delete failed: %s", strings.TrimSpace(string(raw)))
	}
	return nil
}

func (s *GitLabStorage) ListByDate(ctx context.Context, date string) (map[string][]FileInfo, error) {
	if date == "" {
		return s.listAllByTree(ctx)
	}
	targetDate := date
	repoPath := s.repoPath(targetDate)
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/repository/tree?path=%s&ref=%s&per_page=100", s.baseURL, url.PathEscape(s.project), url.QueryEscape(repoPath), url.QueryEscape(s.branch))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	s.applyHeaders(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return map[string][]FileInfo{}, nil
	}
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gitlab list failed: %s", strings.TrimSpace(string(raw)))
	}

	var entries []struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}

	result := map[string][]FileInfo{}
	for _, e := range entries {
		if e.Type != "blob" {
			continue
		}
		key := strings.TrimPrefix(e.Path, strings.Trim(s.pathPrefix, "/")+"/")
		key = strings.TrimPrefix(key, "/")
		result[targetDate] = append(result[targetDate], FileInfo{
			Name:      filepath.Base(key),
			Path:      key,
			Size:      0,
			DirectURL: s.ResolveDirectURL(key),
		})
	}
	return result, nil
}

func (s *GitLabStorage) listAllByTree(ctx context.Context) (map[string][]FileInfo, error) {
	rootPath := s.pathPrefix
	endpoint := fmt.Sprintf("%s/api/v4/projects/%s/repository/tree?path=%s&ref=%s&per_page=100&recursive=true", s.baseURL, url.PathEscape(s.project), url.QueryEscape(rootPath), url.QueryEscape(s.branch))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	s.applyHeaders(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gitlab list tree failed: %s", strings.TrimSpace(string(raw)))
	}

	var entries []struct {
		Path string `json:"path"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}

	result := map[string][]FileInfo{}
	prefix := strings.Trim(s.pathPrefix, "/")
	for _, e := range entries {
		if e.Type != "blob" {
			continue
		}
		key := e.Path
		if prefix != "" {
			if !strings.HasPrefix(key, prefix+"/") {
				continue
			}
			key = strings.TrimPrefix(key, prefix+"/")
		}
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			continue
		}
		day := parts[0]
		result[day] = append(result[day], FileInfo{
			Name:      filepath.Base(key),
			Path:      key,
			DirectURL: s.ResolveDirectURL(key),
		})
	}
	return result, nil
}

func (s *GitLabStorage) Open(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	u := s.rawEndpoint(s.repoPath(key))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", 0, err
	}
	s.applyHeaders(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return nil, "", 0, os.ErrNotExist
		}
		return nil, "", 0, fmt.Errorf("gitlab open failed: %s", resp.Status)
	}
	return resp.Body, resp.Header.Get("Content-Type"), resp.ContentLength, nil
}

func (s *GitLabStorage) ResolveDirectURL(key string) string {
	return s.rawEndpoint(s.repoPath(key))
}

func (s *GitLabStorage) repoPath(key string) string {
	key = strings.TrimPrefix(key, "/")
	if s.pathPrefix == "" {
		return key
	}
	return path.Join(s.pathPrefix, key)
}

func (s *GitLabStorage) fileEndpoint(repoPath string) string {
	return fmt.Sprintf("%s/api/v4/projects/%s/repository/files/%s", s.baseURL, url.PathEscape(s.project), url.PathEscape(repoPath))
}

func (s *GitLabStorage) rawEndpoint(repoPath string) string {
	return fmt.Sprintf("%s/api/v4/projects/%s/repository/files/%s/raw?ref=%s", s.baseURL, url.PathEscape(s.project), url.PathEscape(repoPath), url.QueryEscape(s.branch))
}

func (s *GitLabStorage) applyHeaders(req *http.Request) {
	req.Header.Set("PRIVATE-TOKEN", s.token)
	req.Header.Set("Content-Type", "application/json")
}

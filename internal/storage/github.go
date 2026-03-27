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

type GitHubStorage struct {
	owner      string
	repo       string
	branch     string
	pathPrefix string
	token      string
	client     *http.Client
}

func init() {
	Register("github", func(cfg DriverConfig) (Storage, error) {
		return NewGitHubStorage(cfg.GitHub)
	})
}

func NewGitHubStorage(cfg GitHubConfig) (*GitHubStorage, error) {
	if cfg.Owner == "" || cfg.Repo == "" {
		return nil, fmt.Errorf("github owner/repo is required")
	}
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	token := cfg.Token
	if token == "" && cfg.TokenEnv != "" {
		token = strings.TrimSpace(os.Getenv(cfg.TokenEnv))
	}
	if token == "" {
		return nil, fmt.Errorf("github token is required")
	}

	return &GitHubStorage{
		owner:      cfg.Owner,
		repo:       cfg.Repo,
		branch:     cfg.Branch,
		pathPrefix: strings.Trim(cfg.PathPrefix, "/"),
		token:      token,
		client:     &http.Client{Timeout: 20 * time.Second},
	}, nil
}

func (s *GitHubStorage) Save(ctx context.Context, r io.Reader, originalFilename string) (StoredObject, error) {
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
	repoPath := s.repoPath(key)
	reqBody := map[string]string{
		"message": "upload image: " + key,
		"content": base64.StdEncoding.EncodeToString(data),
		"branch":  s.branch,
	}
	body, _ := json.Marshal(reqBody)

	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", s.owner, s.repo, escapePathKeepSlash(repoPath))
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
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
		return StoredObject{}, fmt.Errorf("github upload failed: %s", strings.TrimSpace(string(raw)))
	}

	return StoredObject{
		Key:       key,
		DirectURL: s.ResolveDirectURL(key),
		Size:      int64(len(data)),
	}, nil
}

func (s *GitHubStorage) Delete(ctx context.Context, key string) error {
	repoPath := s.repoPath(key)
	sha, err := s.getFileSHA(ctx, repoPath)
	if err != nil {
		return err
	}
	reqBody := map[string]string{
		"message": "delete image: " + key,
		"sha":     sha,
		"branch":  s.branch,
	}
	body, _ := json.Marshal(reqBody)

	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", s.owner, s.repo, escapePathKeepSlash(repoPath))
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, bytes.NewReader(body))
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
		return fmt.Errorf("github delete failed: %s", strings.TrimSpace(string(raw)))
	}
	return nil
}

func (s *GitHubStorage) ListByDate(ctx context.Context, date string) (map[string][]FileInfo, error) {
	if date == "" {
		return s.listAllByTree(ctx)
	}
	targetDate := date
	repoPath := s.repoPath(targetDate)
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", s.owner, s.repo, escapePathKeepSlash(repoPath), url.QueryEscape(s.branch))

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
		return nil, fmt.Errorf("github list failed: %s", strings.TrimSpace(string(raw)))
	}

	var entries []struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		Size        int64  `json:"size"`
		DownloadURL string `json:"download_url"`
		Type        string `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, err
	}

	result := map[string][]FileInfo{}
	for _, e := range entries {
		if e.Type != "file" {
			continue
		}
		day := targetDate
		key := strings.TrimPrefix(e.Path, strings.Trim(s.pathPrefix, "/")+"/")
		key = strings.TrimPrefix(key, "/")
		result[day] = append(result[day], FileInfo{
			Name:      filepath.Base(key),
			Path:      key,
			Size:      e.Size,
			DirectURL: e.DownloadURL,
		})
	}
	return result, nil
}

func (s *GitHubStorage) listAllByTree(ctx context.Context) (map[string][]FileInfo, error) {
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", s.owner, s.repo, url.PathEscape(s.branch))
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
		return nil, fmt.Errorf("github list tree failed: %s", strings.TrimSpace(string(raw)))
	}

	var payload struct {
		Tree []struct {
			Path string `json:"path"`
			Type string `json:"type"`
			Size int64  `json:"size"`
		} `json:"tree"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	result := map[string][]FileInfo{}
	prefix := strings.Trim(s.pathPrefix, "/")
	for _, item := range payload.Tree {
		if item.Type != "blob" {
			continue
		}
		key := item.Path
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
			Size:      item.Size,
			DirectURL: s.ResolveDirectURL(key),
		})
	}
	return result, nil
}

func (s *GitHubStorage) Open(ctx context.Context, key string) (io.ReadCloser, string, int64, error) {
	u := s.ResolveDirectURL(key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", 0, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "", 0, err
	}
	if resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, "", 0, fmt.Errorf("github open failed: %s", resp.Status)
	}
	return resp.Body, resp.Header.Get("Content-Type"), resp.ContentLength, nil
}

func (s *GitHubStorage) ResolveDirectURL(key string) string {
	repoPath := s.repoPath(key)
	return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", s.owner, s.repo, s.branch, repoPath)
}

func (s *GitHubStorage) repoPath(key string) string {
	key = strings.TrimPrefix(key, "/")
	if s.pathPrefix == "" {
		return key
	}
	return path.Join(s.pathPrefix, key)
}

func (s *GitHubStorage) getFileSHA(ctx context.Context, repoPath string) (string, error) {
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", s.owner, s.repo, escapePathKeepSlash(repoPath), url.QueryEscape(s.branch))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	s.applyHeaders(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", os.ErrNotExist
	}
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("github get file failed: %s", strings.TrimSpace(string(raw)))
	}
	var payload struct {
		SHA string `json:"sha"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if payload.SHA == "" {
		return "", fmt.Errorf("missing github file sha")
	}
	return payload.SHA, nil
}

func (s *GitHubStorage) applyHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")
}

func withTimePrefix(filename string) string {
	return time.Now().Format("150405") + "-" + filename
}

func escapePathKeepSlash(p string) string {
	return strings.ReplaceAll(url.PathEscape(p), "%2F", "/")
}


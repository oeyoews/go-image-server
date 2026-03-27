package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type LocalStorage struct {
	baseDir string
}

func init() {
	Register("local", func(cfg DriverConfig) (Storage, error) {
		return NewLocalStorage(cfg.Local.BaseDir)
	})
}

func NewLocalStorage(baseDir string) (*LocalStorage, error) {
	if baseDir == "" {
		baseDir = "./uploads"
	}
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}
	return &LocalStorage{baseDir: baseDir}, nil
}

func (s *LocalStorage) Save(_ context.Context, r io.Reader, originalFilename string) (StoredObject, error) {
	filename, ext, err := ensureAllowedFilename(originalFilename)
	if err != nil {
		return StoredObject{}, err
	}

	dateDir := todayDir()
	dir := filepath.Join(s.baseDir, dateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return StoredObject{}, fmt.Errorf("create date dir: %w", err)
	}

	target := filepath.Join(dir, filename)
	for i := 1; ; i++ {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			break
		}
		nameOnly := strings.TrimSuffix(filename, ext)
		target = filepath.Join(dir, fmt.Sprintf("%s-%d%s", nameOnly, i, ext))
	}

	if err := s.securePath(target, dir); err != nil {
		return StoredObject{}, err
	}

	f, err := os.Create(target)
	if err != nil {
		return StoredObject{}, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	written, err := io.Copy(f, io.LimitReader(r, MaxFileSize+1))
	if err != nil {
		os.Remove(target)
		return StoredObject{}, fmt.Errorf("write file: %w", err)
	}
	if written > MaxFileSize {
		os.Remove(target)
		return StoredObject{}, fmt.Errorf("file too large")
	}

	rel := filepath.Join(dateDir, filepath.Base(target))
	return StoredObject{
		Key:       rel,
		DirectURL: "",
		Size:      written,
	}, nil
}

// Delete 删除相对路径为 relPath 的图片文件。
// relPath 形如 "2026-03-14/xxx.png"。
func (s *LocalStorage) Delete(_ context.Context, key string) error {
	if strings.Contains(key, "..") {
		return fmt.Errorf("invalid path")
	}
	full := filepath.Join(s.baseDir, key)

	dayDir := filepath.Dir(full)
	if err := s.securePath(full, s.baseDir); err != nil {
		return err
	}

	if err := os.Remove(full); err != nil {
		return err
	}

	// 尝试删除空的日期目录（忽略错误）
	_ = os.Remove(dayDir)
	return nil
}

// ListByDate 返回按日期分组的文件列表。
// 如果 date 非空，则只返回该日期（YYYY-MM-DD）的文件。
func (s *LocalStorage) ListByDate(_ context.Context, date string) (map[string][]FileInfo, error) {
	result := make(map[string][]FileInfo)

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return result, err
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		day := e.Name()
		if date != "" && day != date {
			continue
		}

		dir := filepath.Join(s.baseDir, day)
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() {
				continue
			}
			info, err := f.Info()
			if err != nil {
				continue
			}
			name := f.Name()
			relPath := filepath.Join(day, name)
			result[day] = append(result[day], FileInfo{
				Name: name,
				Path: relPath,
				Size: info.Size(),
			})
		}
	}

	// 按文件名排序，稳定输出
	for day := range result {
		sort.Slice(result[day], func(i, j int) bool {
			return result[day][i].Name < result[day][j].Name
		})
	}

	return result, nil
}

func (s *LocalStorage) Open(_ context.Context, key string) (io.ReadCloser, string, int64, error) {
	if strings.Contains(key, "..") {
		return nil, "", 0, fmt.Errorf("invalid path")
	}
	full := filepath.Join(s.baseDir, key)
	if err := s.securePath(full, s.baseDir); err != nil {
		return nil, "", 0, err
	}

	f, err := os.Open(full)
	if err != nil {
		return nil, "", 0, err
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, "", 0, err
	}

	ext := strings.ToLower(filepath.Ext(key))
	contentType, ok := allowedExts[ext]
	if !ok {
		contentType = "application/octet-stream"
	}

	return f, contentType, info.Size(), nil
}

func (s *LocalStorage) ResolveDirectURL(_ string) string {
	return ""
}

func (s *LocalStorage) securePath(path, base string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(abs, baseAbs) {
		return fmt.Errorf("path traversal detected")
	}
	return nil
}

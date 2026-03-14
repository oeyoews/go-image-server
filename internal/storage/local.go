package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	MaxFileSize = 5 << 20 // 5MB
)

var allowedExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".svg":  "image/svg+xml",
}

type LocalStorage struct {
	baseDir string
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

var unsafeFilenameChars = regexp.MustCompile(`[\/\\:*?"<>|]`)

func sanitizeFilename(name string) string {
	base := filepath.Base(name)
	return unsafeFilenameChars.ReplaceAllString(base, "_")
}

func (s *LocalStorage) Save(r io.Reader, originalFilename string) (string, error) {
	ext := strings.ToLower(filepath.Ext(originalFilename))
	if _, ok := allowedExts[ext]; !ok {
		return "", fmt.Errorf("unsupported extension: %s", ext)
	}

	dateDir := time.Now().Format("2006-01-02")
	dir := filepath.Join(s.baseDir, dateDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create date dir: %w", err)
	}

	sanitized := sanitizeFilename(originalFilename)
	if sanitized == "" || sanitized == "." || sanitized == ".." {
		sanitized = time.Now().Format("150405") + ext
	}

	filename := sanitized
	if !strings.HasSuffix(strings.ToLower(filename), ext) {
		filename += ext
	}

	// 防止同名覆盖，简单追加序号
	target := filepath.Join(dir, filename)
	for i := 1; ; i++ {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			break
		}
		nameOnly := strings.TrimSuffix(filename, ext)
		target = filepath.Join(dir, fmt.Sprintf("%s-%d%s", nameOnly, i, ext))
	}

	if err := s.securePath(target, dir); err != nil {
		return "", err
	}

	f, err := os.Create(target)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	limited := io.LimitReader(r, MaxFileSize)
	if _, err := io.Copy(f, limited); err != nil {
		os.Remove(target)
		return "", fmt.Errorf("write file: %w", err)
	}

	rel := filepath.Join(dateDir, filepath.Base(target))
	return rel, nil
}

// Delete 删除相对路径为 relPath 的图片文件。
// relPath 形如 "2026-03-14/xxx.png"。
func (s *LocalStorage) Delete(relPath string) error {
	if strings.Contains(relPath, "..") {
		return fmt.Errorf("invalid path")
	}
	full := filepath.Join(s.baseDir, relPath)

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

type FileInfo struct {
	Name string
	Path string
	Size int64
}

// ListByDate 返回按日期分组的文件列表。
// 如果 date 非空，则只返回该日期（YYYY-MM-DD）的文件。
func (s *LocalStorage) ListByDate(date string) (map[string][]FileInfo, error) {
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

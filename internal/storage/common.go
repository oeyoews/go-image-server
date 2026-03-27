package storage

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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

var unsafeFilenameChars = regexp.MustCompile(`[\/\\:*?"<>|]`)

func sanitizeFilename(name string) string {
	base := filepath.Base(name)
	return unsafeFilenameChars.ReplaceAllString(base, "_")
}

func ensureAllowedFilename(originalFilename string) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(originalFilename))
	if _, ok := allowedExts[ext]; !ok {
		return "", "", fmt.Errorf("unsupported extension: %s", ext)
	}

	sanitized := sanitizeFilename(originalFilename)
	if sanitized == "" || sanitized == "." || sanitized == ".." {
		sanitized = time.Now().Format("150405") + ext
	}
	if !strings.HasSuffix(strings.ToLower(sanitized), ext) {
		sanitized += ext
	}
	return sanitized, ext, nil
}

func todayDir() string {
	return time.Now().Format("2006-01-02")
}

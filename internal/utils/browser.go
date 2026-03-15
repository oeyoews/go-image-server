package utils

import (
	"os/exec"
	"runtime"
	"time"

	log "github.com/sirupsen/logrus"
)

// OpenBrowser 尝试在当前操作系统上使用默认浏览器打开给定 URL。
// 失败时只记录告警日志，不会中断主进程。
func OpenBrowser(url string) {
	time.Sleep(300 * time.Millisecond)

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


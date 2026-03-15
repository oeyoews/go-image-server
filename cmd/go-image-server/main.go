package main

// @title           Go Image Server API
// @version         1.0
// @description     本地图片上传与管理服务的开发文档。
// @BasePath        /api/v1

import (
	"fmt"
	"os"
)

// Version 版本号，构建时可覆盖: go build -ldflags "-X main.Version=1.0.0"
var Version = "1.0.0"

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "-version" || arg == "--version" || arg == "-v" {
			fmt.Println(Version)
			os.Exit(0)
		}
	}
	run()
}

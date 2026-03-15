# Go Image Server

本地图片上传与管理服务，基于 Gin 的轻量级 HTTP 服务，支持 Web 上传页、REST API、按日期存储与静态访问。

## 功能特性

- **Web 上传页**：浏览器打开根路径即可使用，Vue3 + Tailwind CSS（CDN），支持拖拽、粘贴（Ctrl+V）、修改保存名、复制 Markdown/Wikitext/链接、按日期查看与删除
- **图片上传**：支持 JPG、PNG、GIF、WebP、BMP、SVG，单文件最大 5MB
- **按日期存储**：自动按 `YYYY-MM-DD` 分目录保存
- **静态访问**：通过 `/files/日期/文件名` 直接访问已上传图片
- **图片列表**：按日期分组，支持 `?date=YYYY-MM-DD` 过滤
- **删除图片**：通过相对路径删除指定图片
- **CORS**：默认开启跨域，便于前端或 TiddlyWiki 等调用
- **端口自动递增**：若指定端口被占用，自动尝试下一端口（最多 10 次）

## 技术栈

- **Go 1.25+**
- [Gin](https://github.com/gin-gonic/gin) - HTTP 框架
- [logrus](https://github.com/sirupsen/logrus) - 日志
- [gin-contrib/cors](https://github.com/gin-contrib/cors) - 跨域
- **Web 页**：Vue 3、Tailwind CSS（CDN），单文件 `web/index.html`，浅色主题

## 快速开始

### 安装依赖

```bash
go mod download
```

### 运行

```bash
go run ./cmd/go-image-server
```

或使用 Go Modules 安装后直接运行可执行文件。

默认监听 `http://localhost:48083`。首次运行会在配置目录生成默认 `config.json`（见下方配置说明）。

### Web 上传页

启动后浏览器访问：

```
http://localhost:48083/
```

即可使用上传界面：选择/拖拽/粘贴图片、填写保存名、上传后复制 Markdown 或链接，并可查看按日期分组的图片列表、复制链接或删除。

### 使用 Make 构建

```bash
# 当前平台
make build
# 输出: bin/go-image-server

# 多平台 (windows/linux/darwin, amd64/arm64)
make build-all
```

## 配置说明

配置文件路径（按优先级）：

1. 环境变量 `CONFIG_FILE` 指定路径
2. 用户配置目录：`$XDG_CONFIG_HOME/go-image-server/config.json`（Linux/macOS）或 `%AppData%\go-image-server\config.json`（Windows）
3. 当前目录 `config.json`

示例 `config.example.json`（仓库内 `configs/config.example.json`）：

```json
{
  "port": "48083",
  "upload_dir": "",
  "open_browser": true
}
```

| 字段         | 说明                          |
|--------------|-------------------------------|
| `port`       | 服务端口，默认 `48083`         |
| `upload_dir` | 上传根目录，为空时见下方解析  |

**上传目录解析顺序**：

1. 环境变量 `UPLOAD_DIR`（支持 `~` 家目录）
2. 配置文件中的 `upload_dir`
3. 用户缓存目录：`$XDG_CACHE_HOME/go-image-server/uploads` 或 `%LocalAppData%\go-image-server\uploads`
4. 当前目录 `./uploads`

## 环境变量

| 变量          | 说明                     |
|---------------|--------------------------|
| `CONFIG_FILE` | 配置文件路径             |
| `UPLOAD_DIR`  | 上传根目录（覆盖配置）   |
| `PORT`        | 服务端口（覆盖配置）    |
| `LOG_LEVEL`   | 日志级别：debug/info/warn/error |

## API 说明

所有接口前缀为 **`/api/v1`**，响应格式统一为 `{ "code": number, "message": string, "data": ... }`。

| 方法   | 路径              | 说明 |
|--------|-------------------|------|
| GET    | `/api/v1/info`    | 返回 `upload_dir`、`config_file`、`version` |
| POST   | `/api/v1/upload`  | 上传图片，form 字段 `file` 必填，可选 `filename` 指定保存名 |
| GET    | `/api/v1/images`  | 图片列表，可选 query `date=YYYY-MM-DD` |
| DELETE | `/api/v1/images`  | 删除图片，query `path=YYYY-MM-DD/xxx.png` |
| GET    | `/files/*`        | 静态访问，对应上传目录中的文件 |
| GET    | `/`               | Web 上传页（`web/index.html`） |

### 上传示例

```bash
curl -X POST http://localhost:48083/api/v1/upload \
  -F "file=@/path/to/image.png" \
  -F "filename=my-image.png"
```

响应示例（`data` 字段）：

```json
{
  "code": 201,
  "message": "success",
  "data": {
    "url": "http://localhost:48083/files/2026-03-15/my-image.png",
    "path": "2026-03-15/my-image.png"
  }
}
```

### 列表示例

```bash
# 全部
curl http://localhost:48083/api/v1/images

# 指定日期
curl "http://localhost:48083/api/v1/images?date=2026-03-15"
```

### 删除示例

```bash
curl -X DELETE "http://localhost:48083/api/v1/images?path=2026-03-15/my-image.png"
```

## 项目结构

基于 [Go 项目标准布局](https://raw.githubusercontent.com/golang-standards/project-layout/master/README_zh-CN.md) 进行组织：

```
go-image-server/
├── cmd/
│   └── go-image-server/   # 主应用入口
│       ├── main.go        # 入口、配置加载、路由
│       ├── swagger_dev.go # dev 构建下的 Swagger UI
│       └── swagger_stub.go# 非 dev 构建的空实现
├── internal/              # 私有应用代码
│   ├── handler/
│   │   ├── upload.go      # 上传/列表/删除/Info 处理
│   │   └── response.go    # 统一响应格式
│   ├── storage/
│   │   └── local.go       # 本地存储、按日期目录、安全路径
│   └── shortid/
│       └── shortid.go     # Base62 短 ID（可选用途）
├── configs/
│   └── config.example.json# 配置示例
├── docs/                  # Swagger 文档
│   ├── docs.go
│   ├── swagger.json
│   └── swagger.yaml
├── web/
│   └── index.html         # Web 上传页（Vue3 + Tailwind CDN）
├── scripts/               # 构建与工具脚本
│   ├── build.ps1          # Windows 单平台构建
│   ├── build-all.ps1      # Windows 多平台构建
│   └── swag-init.ps1      # 生成 Swagger 文档
├── Makefile               # 多平台构建（使用 ./cmd/go-image-server）
├── go.mod / go.sum        # Go 模块与依赖
└── README.md
```

## 安全说明

- 上传文件名会过滤 `\/:*?"<>|` 等不安全字符
- 删除与访问均做路径校验，防止目录穿越
- 仅允许配置的图片扩展名与 MIME 类型

## License

MIT

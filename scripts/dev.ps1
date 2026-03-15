param(
    [string]$Port
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

# 当前脚本所在目录（scripts/）
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
# 仓库根目录：scripts 的上一级
$repoRoot = Resolve-Path (Join-Path $scriptDir "..")

# 先生成 Swagger 文档（api/docs.go、swagger.yaml/json）
Write-Host "==> Generating Swagger docs (swag-init.ps1)..." -ForegroundColor Cyan
& (Join-Path $scriptDir "swag-init.ps1")

# 设置开发环境
$env:APP_ENV = "dev"

if ($Port) {
    $env:PORT = $Port
}

Write-Host "APP_ENV=dev PORT=$($env:PORT)" -ForegroundColor Green

# 使用 dev 构建标签启用 Swagger 相关代码（在仓库根目录运行 go 命令）
Push-Location $repoRoot
try {
    go run -tags dev ./cmd/go-image-server
} finally {
    Pop-Location
}


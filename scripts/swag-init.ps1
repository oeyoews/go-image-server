param(
    [string]$MainFile = "cmd/go-image-server/main.go",
    [string]$OutputDir = "api"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

Write-Host "==> Checking for 'swag' CLI..." -ForegroundColor Cyan
$swag = Get-Command "swag" -ErrorAction SilentlyContinue
if (-not $swag) {
    Write-Error "'swag' 命令未找到，请先安装 swag CLI: go install github.com/swaggo/swag/cmd/swag@latest"
    exit 1
}

Write-Host "==> Running 'swag init'..." -ForegroundColor Cyan
swag init -g $MainFile -o $OutputDir --outputTypes "go,yaml,json"

if ($LASTEXITCODE -ne 0) {
    Write-Error "swag init 失败，退出码: $LASTEXITCODE"
    exit $LASTEXITCODE
}

Write-Host "==> swag init 完成，输出目录: '$OutputDir'" -ForegroundColor Green


param(
    [string]$OutputDir = "bin",
    [string]$BinaryName = "go-image-server",
    [switch]$NoCompress
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

Write-Host "==> Cleaning output directory '$OutputDir'..." -ForegroundColor Cyan
if (Test-Path $OutputDir) {
    Remove-Item -Recurse -Force $OutputDir
}
New-Item -ItemType Directory -Path $OutputDir | Out-Null

Write-Host "==> Running go mod tidy..." -ForegroundColor Cyan
go mod tidy

Write-Host "==> Building project (cmd/go-image-server)..." -ForegroundColor Cyan
$env:CGO_ENABLED = "0"
if ($NoCompress) {
    Write-Host "==> Compression disabled (NoCompress)..." -ForegroundColor Yellow
	go build -o (Join-Path $OutputDir "$BinaryName.exe") ./cmd/$BinaryName
} else {
    $ldflags = "-s -w"
    Write-Host "==> Using ldflags '$ldflags' with -trimpath for smaller binary..." -ForegroundColor Cyan
	go build -trimpath -ldflags $ldflags -o (Join-Path $OutputDir "$BinaryName.exe") ./cmd/$BinaryName
}

if ($LASTEXITCODE -ne 0) {
    Write-Error "Build failed with exit code $LASTEXITCODE"
    exit $LASTEXITCODE
}

Write-Host "==> Build succeeded. Output: $(Join-Path $OutputDir "$BinaryName.exe")" -ForegroundColor Green

# 显示构建后二进制大小，便于和不同参数对比
$binPath = Join-Path $OutputDir "$BinaryName.exe"
if (Test-Path $binPath) {
    $sizeBytes = (Get-Item $binPath).Length
    $sizeMB = $sizeBytes / 1MB
    Write-Host ("==> Binary size: {0:N2} MB ({1} bytes)" -f $sizeMB, $sizeBytes) -ForegroundColor Green
}


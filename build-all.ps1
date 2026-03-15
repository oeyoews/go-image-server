param(
    [string]$OutputDir = "bin",
    [string]$BinaryName = "go-image-server",
    [switch]$NoCompress
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$GOOS_LIST = @("windows", "linux", "darwin")
$GOARCH_LIST = @("amd64", "arm64")
$LDFLAGS = "-s -w"

Write-Host "==> Cleaning output directory '$OutputDir'..." -ForegroundColor Cyan
if (Test-Path $OutputDir) {
    Remove-Item -Recurse -Force $OutputDir
}
New-Item -ItemType Directory -Path $OutputDir | Out-Null

Write-Host "==> Running go mod tidy..." -ForegroundColor Cyan
go mod tidy
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$env:CGO_ENABLED = "0"
if (-not $NoCompress) {
    Write-Host "==> Using ldflags '$LDFLAGS' with -trimpath..." -ForegroundColor Cyan
}

foreach ($goos in $GOOS_LIST) {
    foreach ($goarch in $GOARCH_LIST) {
        $ext = if ($goos -eq "windows") { ".exe" } else { "" }
        $outName = "${BinaryName}-${goos}-${goarch}${ext}"
        $outPath = Join-Path $OutputDir $outName
        Write-Host "==> Building $goos/$goarch -> $outName" -ForegroundColor Cyan
        $env:GOOS = $goos
        $env:GOARCH = $goarch
        if ($NoCompress) {
            go build -trimpath -o $outPath .
        } else {
            go build -trimpath -ldflags $LDFLAGS -o $outPath .
        }
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Build failed: $goos/$goarch"
            exit $LASTEXITCODE
        }
    }
}

Write-Host "==> Build-all succeeded. Output: $OutputDir" -ForegroundColor Green
Get-ChildItem $OutputDir | ForEach-Object { Write-Host "    $($_.Name)" }

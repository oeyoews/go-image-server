param(
    [string]$Port
)

# 设置开发环境
$env:APP_ENV = "dev"

if ($Port) {
    $env:PORT = $Port
}

Write-Host "APP_ENV=dev PORT=$($env:PORT)" -ForegroundColor Green

# 使用 dev 构建标签启用 Swagger 相关代码
go run -tags dev .


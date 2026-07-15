# Windows 编译（保留 build\bin\data 与 logs：不使用 --clean）
$ErrorActionPreference = "Stop"
Set-Location (Join-Path $PSScriptRoot "..")

Write-Host "Start building..."
Stop-Process -Name go-stock -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

# 使用用户目录 Go 缓存，避免沙箱/临时 GOMODCACHE 导致依赖解析失败
if (-not $env:GOMODCACHE) { $env:GOMODCACHE = Join-Path $env:USERPROFILE "go\pkg\mod" }
if (-not $env:GOCACHE) { $env:GOCACHE = Join-Path $env:USERPROFILE "AppData\Local\go-build" }

if (-not $env:GOCACHE) { $env:GOCACHE = Join-Path $env:USERPROFILE "AppData\Local\go-build" }

Write-Host "Building signal scan JS bundle..."
Push-Location (Join-Path $PSScriptRoot "..\frontend")
npx --yes esbuild ../scripts/scansignals/scan-batch.ts --bundle --platform=neutral --format=iife --global-name=SignalScanBatch --outfile=../backend/data/signal_scan_bundle.js
if ($LASTEXITCODE -ne 0) { Pop-Location; exit $LASTEXITCODE }
Pop-Location

wails build --platform windows/amd64
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
Write-Host "Build finished."

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

# --- Beta release identity (Version Domain) ---
# Priority at runtime: data/release.json (non-empty fields) then these ldflags via Bootstrap/ApplyLegacy.
# Keep data/release.json build_time/git_commit empty so ldflags are not clobbered.
$releaseVersion = if ($env:GOSTOCK_VERSION) { $env:GOSTOCK_VERSION } else { "0.1.0-beta" }
$releaseChannel = if ($env:GOSTOCK_CHANNEL) { $env:GOSTOCK_CHANNEL } else { "beta" }
$releaseBuildMode = if ($env:GOSTOCK_BUILD_MODE) { $env:GOSTOCK_BUILD_MODE } else { "production" }
$releaseCommit = "unknown"
try {
  $releaseCommit = (git rev-parse --short HEAD 2>$null)
  if (-not $releaseCommit) { $releaseCommit = "unknown" }
  $dirty = (git status --porcelain 2>$null)
  if ($dirty) { $releaseCommit = "$releaseCommit-dirty" }
} catch {
  $releaseCommit = "unknown"
}
$releaseBuildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
Write-Host "Release identity: version=$releaseVersion build_time=$releaseBuildTime git_commit=$releaseCommit channel=$releaseChannel build_mode=$releaseBuildMode"

$ldflags = @(
  "-X 'go-stock/backend/version.Version=$releaseVersion'"
  "-X 'go-stock/backend/version.BuildTime=$releaseBuildTime'"
  "-X 'go-stock/backend/version.GitCommit=$releaseCommit'"
  "-X 'go-stock/backend/version.Channel=$releaseChannel'"
  "-X 'go-stock/backend/version.BuildMode=$releaseBuildMode'"
  "-X 'main.Version=$releaseVersion'"
  "-X 'main.VersionCommit=$releaseCommit'"
) -join " "

Write-Host "Building signal scan JS bundle..."
Push-Location (Join-Path $PSScriptRoot "..\frontend")
npx --yes esbuild ../scripts/scansignals/scan-batch.ts --bundle --platform=neutral --format=iife --global-name=SignalScanBatch --outfile=../backend/data/signal_scan_bundle.js
if ($LASTEXITCODE -ne 0) { Pop-Location; exit $LASTEXITCODE }
Pop-Location

wails build --platform windows/amd64 -ldflags "$ldflags"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

# Sync Beta runtime JSON into build/bin/data (exe cwd).
& (Join-Path $PSScriptRoot "sync-beta-runtime-data.ps1")

Write-Host "Build finished."

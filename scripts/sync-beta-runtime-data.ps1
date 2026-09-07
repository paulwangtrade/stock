# Sync repo data/*.json -> build/bin/data/ (Beta runtime cwd; read-only copy).
$ErrorActionPreference = "Stop"
$root = Join-Path $PSScriptRoot ".."
$src = Join-Path $root "data"
$dst = Join-Path $root "build\bin\data"

if (-not (Test-Path (Join-Path $root "build\bin"))) {
  Write-Host "skip sync-beta-runtime-data: build\bin not found (run wails build first)"
  exit 0
}

if (-not (Test-Path -LiteralPath $src -PathType Container)) {
  Write-Warning "skip sync-beta-runtime-data: source directory missing: $src"
  exit 0
}

New-Item -ItemType Directory -Force -Path $dst | Out-Null

$jsonFiles = @(Get-ChildItem -LiteralPath $src -Filter "*.json" -File -ErrorAction SilentlyContinue)
if ($jsonFiles.Count -eq 0) {
  Write-Warning "sync-beta-runtime-data: no JSON files in $src"
  exit 0
}

Write-Host ""
Write-Host "=== Beta data sync ===" -ForegroundColor Cyan
Write-Host "source: $src"
Write-Host "target: $dst"
Write-Host ""

$synced = 0
foreach ($file in ($jsonFiles | Sort-Object Name)) {
  $target = Join-Path $dst $file.Name
  Copy-Item -LiteralPath $file.FullName -Destination $target -Force
  Write-Host "[sync] $($file.Name) -> build/bin/data/$($file.Name)"
  $synced++
}

Write-Host ""
Write-Host "=== sync complete: $synced file(s) copied ===" -ForegroundColor Green

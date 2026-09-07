# Generate go-stock license inventories and THIRD_PARTY_NOTICES.md
# Engineering-only; does not touch trading/database/strategy code.

param(
    [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path,
    [switch]$SkipFrontend,
    [switch]$SkipSyft
)

$ErrorActionPreference = "Stop"
Set-Location $RepoRoot

$invDir = Join-Path $RepoRoot "compliance\inventory"
New-Item -ItemType Directory -Force -Path $invDir | Out-Null

Write-Host "==> Go module license inventory"
go mod download
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
$goOut = Join-Path $invDir "go-modules.json"
go run ./tools/compliance/goinventory -repo $RepoRoot -out $goOut -forbidden "GPL,AGPL,LGPL"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$goJson = Get-Content $goOut -Raw | ConvertFrom-Json
if ($goJson.violations -and $goJson.violations.Count -gt 0) {
    Write-Warning "Go inventory reported $($goJson.violations.Count) policy violation(s). Review compliance/policy.yaml allowlist."
    $goJson.violations | ForEach-Object { Write-Warning "  $_" }
}

if (-not $SkipFrontend) {
    Write-Host "==> Frontend production license inventory"
    $feRoot = Join-Path $RepoRoot "frontend"
    $licTools = Join-Path $PSScriptRoot "."
    if (-not (Test-Path (Join-Path $feRoot "node_modules"))) {
        Write-Host "    npm ci in frontend (node_modules missing)"
        Push-Location $feRoot
        npm ci
        Pop-Location
    }
    if (-not (Test-Path (Join-Path $licTools "node_modules"))) {
        Write-Host "    npm install in scripts/license"
        Push-Location $licTools
        npm install
        Pop-Location
    }
    $feOut = Join-Path $invDir "frontend-packages.json"
    Push-Location $feRoot
    npx --prefix $licTools license-checker --production --json --out $feOut
    Pop-Location
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    $feJson = Get-Content $feOut -Raw | ConvertFrom-Json
    $feProps = $feJson.PSObject.Properties
    $feViolations = @()
    foreach ($p in $feProps) {
        $lic = [string]$p.Value.licenses
        if ($lic -match 'GPL|AGPL|LGPL') {
            $feViolations += "$($p.Name): $lic"
        }
    }
    if ($feViolations.Count -gt 0) {
        Write-Warning "Frontend inventory: $($feViolations.Count) package(s) with copyleft licenses (may be build-only)."
        $feViolations | ForEach-Object { Write-Warning "  $_" }
    }
}

if (-not $SkipSyft) {
    $syft = Get-Command syft -ErrorAction SilentlyContinue
    if ($syft) {
        Write-Host "==> CycloneDX SBOM via syft"
        $sbomOut = Join-Path $invDir "sbom.cyclonedx.json"
        $exe = Join-Path $RepoRoot "build\bin\go-stock.exe"
        if (Test-Path $exe) {
            syft $exe -o cyclonedx-json=$sbomOut
        } else {
            Write-Host "    skip syft binary scan (build\bin\go-stock.exe not found); scanning repo"
            syft dir:$RepoRoot -o cyclonedx-json=$sbomOut
        }
    } else {
        Write-Host "==> syft not installed; skip CycloneDX (optional)"
    }
}

Write-Host "==> Merge THIRD_PARTY_NOTICES.md"
go run ./tools/compliance/mergenotices -repo $RepoRoot -out THIRD_PARTY_NOTICES.md
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "Done. Inventories in compliance/inventory/"

<#
.SYNOPSIS
  Thin wrapper: Phase9-C.3 Daily Runtime Review (read-only).

.EXAMPLE
  powershell -NoProfile -File scripts/phase9_c3_daily_runtime_review.ps1
  powershell -NoProfile -File scripts/phase9_c3_daily_runtime_review.ps1 -TradeDate 2026-08-03
#>
[CmdletBinding()]
param(
  [string]$TradeDate = (Get-Date -Format 'yyyy-MM-dd'),
  [string]$RepoRoot = '',
  [string]$BinDir = '',
  [string]$OutDir = ''
)

$ErrorActionPreference = 'Stop'
if (-not $RepoRoot) {
  $RepoRoot = Split-Path -Parent $PSScriptRoot
}
Set-Location $RepoRoot

$argsList = @('./scripts/phase9c3_daily_review', '-date', $TradeDate, '-repo', $RepoRoot)
if ($BinDir) { $argsList += @('-bin', $BinDir) }
if ($OutDir) { $argsList += @('-out', $OutDir) }

& go run @argsList
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$ymd = $TradeDate.Replace('-', '')
$outPath = Join-Path $RepoRoot ("PHASE9_C3_DAILY_RUNTIME_REVIEW_{0}.md" -f $ymd)
if ($OutDir) { $outPath = Join-Path $OutDir ("PHASE9_C3_DAILY_RUNTIME_REVIEW_{0}.md" -f $ymd) }
Write-Host ("Report: {0}" -f $outPath)

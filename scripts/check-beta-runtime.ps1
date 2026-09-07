<#
.SYNOPSIS
  Beta Runtime Preflight — verify go-stock.exe cwd profile before launch (Phase14-A).

.DESCRIPTION
  Read-only checks against the runtime directory (exe + data/). Does NOT modify configs or DB.

.PARAMETER RuntimeDir
  Directory that contains go-stock.exe and data/. Defaults to the current location.

.EXAMPLE
  Set-Location D:\stock\build\bin
  powershell -NoProfile -ExecutionPolicy Bypass -File ..\..\scripts\check-beta-runtime.ps1

.EXAMPLE
  powershell -NoProfile -ExecutionPolicy Bypass -File scripts\check-beta-runtime.ps1 -RuntimeDir D:\stock\build\bin
#>
[CmdletBinding()]
param(
  [string]$RuntimeDir = (Get-Location).Path
)

$ErrorActionPreference = 'Stop'
$RequiredSchemaVersion = 10

$RuntimeDir = (Resolve-Path -LiteralPath $RuntimeDir).Path
$fail = 0
$results = New-Object System.Collections.Generic.List[object]

function Add-CheckResult {
  param(
    [string]$Name,
    [bool]$Ok,
    [string]$Detail = ''
  )
  $script:results.Add([pscustomobject]@{
      Name   = $Name
      Status = $(if ($Ok) { 'PASS' } else { 'FAIL' })
      Detail = $Detail
    })
  if (-not $Ok) { $script:fail++ }
}

function Write-CheckLine {
  param(
    [string]$Name,
    [bool]$Ok,
    [string]$Detail = ''
  )
  if ($Ok) {
    Write-Host "[PASS] $Name" -ForegroundColor Green
    if ($Detail) { Write-Host "       $Detail" -ForegroundColor DarkGray }
  } else {
    Write-Host "[FAIL] $Name" -ForegroundColor Red
    if ($Detail) { Write-Host "       $Detail" -ForegroundColor DarkGray }
  }
}

function Read-JsonConfig {
  param([string]$Path)
  if (-not (Test-Path -LiteralPath $Path)) { return $null }
  $raw = Get-Content -LiteralPath $Path -Raw -Encoding UTF8
  if ($raw.Length -gt 0 -and [int][char]$raw[0] -eq 0xFEFF) {
    $raw = $raw.Substring(1)
  }
  return $raw | ConvertFrom-Json
}

function Get-SchemaVersionFromSqlite3 {
  param([string]$DbPath)
  $sqlite3 = Get-Command sqlite3 -ErrorAction SilentlyContinue
  if (-not $sqlite3) { return $null }
  $query = "SELECT COALESCE(MAX(version),0) FROM schema_migrations WHERE status='applied';"
  $out = & $sqlite3.Source $DbPath $query 2>$null
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($out)) { return $null }
  return [int]($out.ToString().Trim())
}

function Write-Utf8NoBom {
  param([string]$Path, [string]$Content)
  $utf8 = New-Object System.Text.UTF8Encoding $false
  [System.IO.File]::WriteAllText($Path, $Content, $utf8)
}

function Get-SchemaVersionFromGoProbe {
  param([string]$DbPath)
  $go = Get-Command go -ErrorAction SilentlyContinue
  if (-not $go) { return $null }

  $probeRoot = Join-Path $env:LOCALAPPDATA 'go-stock-beta-preflight-probe'
  New-Item -ItemType Directory -Force -Path $probeRoot | Out-Null

  $mainGo = Join-Path $probeRoot 'main.go'
  if (-not (Test-Path -LiteralPath $mainGo)) {
    Write-Utf8NoBom -Path $mainGo -Content @'
package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "db path required")
		os.Exit(2)
	}
	dsn := "file:" + os.Args[1] + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	defer db.Close()

	var version int
	err = db.QueryRow(`SELECT COALESCE(MAX(version),0) FROM schema_migrations WHERE status='applied'`).Scan(&version)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(4)
	}
	fmt.Print(version)
}
'@
  }

  $goMod = Join-Path $probeRoot 'go.mod'
  if (-not (Test-Path -LiteralPath $goMod)) {
    Write-Utf8NoBom -Path $goMod -Content @"
module go-stock.beta.preflight

go 1.22

require modernc.org/sqlite v1.46.1
"@
  }

  Push-Location $probeRoot
  try {
    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    & go mod tidy 2>&1 | Out-Null
    $out = & go run . $DbPath 2>&1
    $exit = $LASTEXITCODE
    $ErrorActionPreference = $prevEap
    if ($exit -ne 0) { return $null }
    return [int]($out.ToString().Trim())
  } catch {
    return $null
  } finally {
    Pop-Location
  }
}

function Get-DatabaseSchemaVersion {
  param([string]$DbPath)
  if (-not (Test-Path -LiteralPath $DbPath)) {
    return @{ Version = $null; Source = 'missing-db' }
  }

  $viaSqlite3 = Get-SchemaVersionFromSqlite3 -DbPath $DbPath
  if ($null -ne $viaSqlite3) {
    return @{ Version = $viaSqlite3; Source = 'sqlite3' }
  }

  $viaGo = Get-SchemaVersionFromGoProbe -DbPath $DbPath
  if ($null -ne $viaGo) {
    return @{ Version = $viaGo; Source = 'go-probe' }
  }

  return @{ Version = $null; Source = 'unavailable' }
}

Write-Host ''
Write-Host '=== Beta Runtime Preflight ===' -ForegroundColor Cyan
Write-Host "RuntimeDir: $RuntimeDir"
Write-Host "Required schema: >= v$RequiredSchemaVersion"
Write-Host ''

# 1. go-stock.exe
$exePath = Join-Path $RuntimeDir 'go-stock.exe'
$exeOk = Test-Path -LiteralPath $exePath
Add-CheckResult -Name 'go-stock.exe present' -Ok $exeOk -Detail $(if ($exeOk) { $exePath } else { "expected $exePath" })
Write-CheckLine -Name 'go-stock.exe present' -Ok $exeOk -Detail $(if ($exeOk) { $exePath } else { "expected $exePath" })

# 2. data/
$dataDir = Join-Path $RuntimeDir 'data'
$dataOk = Test-Path -LiteralPath $dataDir -PathType Container
Add-CheckResult -Name 'data/ directory exists' -Ok $dataOk -Detail $(if ($dataOk) { $dataDir } else { "expected $dataDir" })
Write-CheckLine -Name 'data/ directory exists' -Ok $dataOk -Detail $(if ($dataOk) { $dataDir } else { "expected $dataDir" })

# 3. DB schema >= v10
$dbPath = Join-Path $dataDir 'stock.db'
$schemaInfo = Get-DatabaseSchemaVersion -DbPath $dbPath
$schemaVersion = $schemaInfo.Version
$schemaSource = $schemaInfo.Source
$schemaOk = $false
$schemaDetail = ''

if ($schemaSource -eq 'missing-db') {
  $schemaDetail = "missing $dbPath"
} elseif ($schemaSource -eq 'unavailable') {
  $schemaDetail = 'cannot read schema (install Go or sqlite3 CLI on PATH)'
} elseif ($schemaVersion -ge $RequiredSchemaVersion) {
  $schemaOk = $true
  $schemaDetail = "current=v$schemaVersion via $schemaSource (required >= v$RequiredSchemaVersion)"
} else {
  $schemaDetail = "current=v$schemaVersion via $schemaSource (required >= v$RequiredSchemaVersion)"
}

Add-CheckResult -Name 'DB schema >= v10' -Ok $schemaOk -Detail $schemaDetail
Write-CheckLine -Name 'DB schema >= v10' -Ok $schemaOk -Detail $schemaDetail

# 4. paper_trading_mvp.json
$mvpPath = Join-Path $dataDir 'paper_trading_mvp.json'
$mvp = Read-JsonConfig -Path $mvpPath
$mvpExists = $null -ne $mvp
$mvpEnable = $false
$mvpFillMode = $false
if ($mvpExists) {
  $mvpEnable = [bool]$mvp.enablePaperTrading
  $mvpFillMode = ([string]$mvp.fillMode).Trim().ToUpperInvariant() -eq 'A'
}
$mvpOk = $mvpExists -and $mvpEnable -and $mvpFillMode
$mvpDetail = if (-not $mvpExists) {
  "missing $mvpPath"
} elseif (-not $mvpEnable -or -not $mvpFillMode) {
  "enablePaperTrading=$($mvp.enablePaperTrading) fillMode=$($mvp.fillMode) (expected true / A)"
} else {
  'enablePaperTrading=true fillMode=A'
}
Add-CheckResult -Name 'paper_trading_mvp.json (Track-B ON, fillMode A)' -Ok $mvpOk -Detail $mvpDetail
Write-CheckLine -Name 'paper_trading_mvp.json (Track-B ON, fillMode A)' -Ok $mvpOk -Detail $mvpDetail

# 5. paper_open_buy.json
$openBuyPath = Join-Path $dataDir 'paper_open_buy.json'
$openBuy = Read-JsonConfig -Path $openBuyPath
$openBuyExists = $null -ne $openBuy
$openBuyOff = $false
if ($openBuyExists) {
  $openBuyOff = -not [bool]$openBuy.enablePaperOpenBuy
}
$openBuyOk = $openBuyExists -and $openBuyOff
$openBuyDetail = if (-not $openBuyExists) {
  "missing $openBuyPath"
} elseif (-not $openBuyOff) {
  "enablePaperOpenBuy=$($openBuy.enablePaperOpenBuy) (expected false)"
} else {
  'enablePaperOpenBuy=false'
}
Add-CheckResult -Name 'paper_open_buy.json (Legacy Track-A OFF)' -Ok $openBuyOk -Detail $openBuyDetail
Write-CheckLine -Name 'paper_open_buy.json (Legacy Track-A OFF)' -Ok $openBuyOk -Detail $openBuyDetail

# 6. trading_automation.json
$autoPath = Join-Path $dataDir 'trading_automation.json'
$auto = Read-JsonConfig -Path $autoPath
$autoExists = $null -ne $auto
$autoManual = $false
if ($autoExists) {
  $autoManual = ([string]$auto.automation_mode).Trim().ToUpperInvariant() -eq 'MANUAL'
}
$autoOk = $autoExists -and $autoManual
$autoDetail = if (-not $autoExists) {
  "missing $autoPath"
} elseif (-not $autoManual) {
  "automation_mode=$($auto.automation_mode) (expected MANUAL)"
} else {
  'automation_mode=MANUAL'
}
Add-CheckResult -Name 'trading_automation.json (manual gate)' -Ok $autoOk -Detail $autoDetail
Write-CheckLine -Name 'trading_automation.json (manual gate)' -Ok $autoOk -Detail $autoDetail

Write-Host ''
Write-Host '=== checklist ===' -ForegroundColor Cyan
foreach ($row in $results) {
  $color = if ($row.Status -eq 'PASS') { 'Green' } else { 'Red' }
  Write-Host ("[{0}] {1}" -f $row.Status, $row.Name) -ForegroundColor $color
  if ($row.Detail) { Write-Host ("       {0}" -f $row.Detail) -ForegroundColor DarkGray }
}

Write-Host ''
Write-Host '=== summary ===' -ForegroundColor Cyan
if ($fail -eq 0) {
  Write-Host 'OVERALL: PASS - Beta runtime profile looks correct. You may start go-stock.exe from this directory.' -ForegroundColor Green
  exit 0
}

Write-Host "OVERALL: FAIL - $fail check(s) failed. Fix runtime data/ before starting go-stock.exe." -ForegroundColor Red
Write-Host 'Hint: cwd must be the directory containing go-stock.exe and data/.' -ForegroundColor Yellow
Write-Host '      Dev sync example: powershell -File scripts/sync-beta-runtime-data.ps1' -ForegroundColor Yellow
exit 1

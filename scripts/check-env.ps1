# go-stock local toolchain acceptance check
# Usage: powershell -ExecutionPolicy Bypass -File .\scripts\check-env.ps1

$ErrorActionPreference = 'Continue'
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

# Refresh Machine+User PATH so newly installed Go/Wails are visible
$machinePath = [Environment]::GetEnvironmentVariable('Path', 'Machine')
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$env:Path = "$machinePath;$userPath"
$goBin = Join-Path $env:LOCALAPPDATA 'Programs\go\bin'
$userGoBin = Join-Path $env:USERPROFILE 'go\bin'
if (Test-Path $goBin) { $env:Path = "$goBin;$env:Path" }
if (Test-Path $userGoBin) { $env:Path = "$userGoBin;$env:Path" }

$fail = 0
$warn = 0

function Write-Check([string]$name, [bool]$ok, [string]$detail = '', [bool]$isWarn = $false) {
    if ($ok) {
        Write-Host "[PASS] $name" -ForegroundColor Green
        if ($detail) { Write-Host "       $detail" -ForegroundColor DarkGray }
    } elseif ($isWarn) {
        $script:warn++
        Write-Host "[WARN] $name" -ForegroundColor Yellow
        if ($detail) { Write-Host "       $detail" -ForegroundColor DarkGray }
    } else {
        $script:fail++
        Write-Host "[FAIL] $name" -ForegroundColor Red
        if ($detail) { Write-Host "       $detail" -ForegroundColor DarkGray }
    }
}

Write-Host ""
Write-Host "=== go-stock env check ===" -ForegroundColor Cyan
Write-Host "Root: $Root"
Write-Host ""

# 1. Git
$git = Get-Command git -ErrorAction SilentlyContinue
if ($git) {
    Write-Check 'Git' $true ((git --version 2>&1 | Out-String).Trim())
} else {
    Write-Check 'Git' $false 'git not found'
}

# 2. Node / npm
$node = Get-Command node -ErrorAction SilentlyContinue
$npm = Get-Command npm -ErrorAction SilentlyContinue
if ($node) {
    $nv = (node -v 2>&1 | Out-String).Trim()
    Write-Check 'Node.js' $true "$nv ($($node.Source))"
    $major = [int](($nv -replace '^v', '').Split('.')[0])
    if ($major -lt 18) {
        Write-Check 'Node.js major >= 18' $false "current $nv" $true
    }
} else {
    Write-Check 'Node.js' $false 'Install from https://nodejs.org/'
}
if ($npm) {
    Write-Check 'npm' $true ((npm -v 2>&1 | Out-String).Trim())
} else {
    Write-Check 'npm' $false 'npm not found'
}

# 3. Go
$go = Get-Command go -ErrorAction SilentlyContinue
$requiredGo = '1.26'
if ($go) {
    $gov = (go version 2>&1 | Out-String).Trim()
    Write-Check 'Go' $true "$gov ($($go.Source))"
    if ($gov -notmatch $requiredGo) {
        Write-Check "Go matches go.mod ($requiredGo)" $false "current: $gov" $true
    } else {
        Write-Check "Go matches go.mod ($requiredGo)" $true
    }
} else {
    Write-Check 'Go' $false "Install Go $requiredGo+ from https://go.dev/dl/ and add to PATH"
}

# 4. Wails
$wails = Get-Command wails -ErrorAction SilentlyContinue
if ($wails) {
    Write-Check 'Wails CLI' $true (((wails version 2>&1 | Out-String).Trim()) + " ($($wails.Source))")
} else {
    Write-Check 'Wails CLI' $false 'Run: go install github.com/wailsapp/wails/v2/cmd/wails@latest'
}

# 5. WebView2
$wv2 = Get-ItemProperty 'HKLM:\SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}' -ErrorAction SilentlyContinue
if (-not $wv2) {
    $wv2 = Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\EdgeUpdate\Clients\{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}' -ErrorAction SilentlyContinue
}
if ($wv2 -and $wv2.pv) {
    Write-Check 'WebView2 Runtime' $true "version=$($wv2.pv)"
} else {
    Write-Check 'WebView2 Runtime' $false 'Missing WebView2; Wails UI may fail to start' $true
}

# 6. Frontend deps
$nm = Test-Path (Join-Path $Root 'frontend\node_modules\vite')
Write-Check 'frontend/node_modules (vite)' $nm $(if ($nm) { 'present' } else { 'Run: Set-Location frontend; npm install' })

# 7. Embed inputs
$embeds = @(
    'frontend\dist\index.html',
    'build\appicon.png',
    'build\app.ico',
    'build\stock_basic.json',
    'build\stock_base_info_hk.json',
    'build\stock_base_info_us.json'
)
$missingEmbed = @()
foreach ($e in $embeds) {
    if (-not (Test-Path (Join-Path $Root $e))) { $missingEmbed += $e }
}
Write-Check 'Wails embed assets' ($missingEmbed.Count -eq 0) $(if ($missingEmbed.Count) { "missing: $($missingEmbed -join ', ')" } else { 'frontend/dist + build OK' })

# 8. go list
if ($go) {
    Write-Host ""
    Write-Host '--- go list ---' -ForegroundColor Cyan
    $list = & go list -m 2>&1 | Out-String
    Write-Check 'go list -m' ($LASTEXITCODE -eq 0) $list.Trim()
}

# 9. Workspace hygiene
Write-Host ""
Write-Host '--- workspace hygiene ---' -ForegroundColor Cyan
$gocache = Test-Path (Join-Path $Root '.gocache')
Write-Check '.gocache absent' (-not $gocache) $(if ($gocache) { 'Delete project .gocache; use system GOCACHE' } else { 'OK' }) $gocache

$dirtyRuntime = @(git status --short --untracked-files=no -- 'build/bin' 'logs' 'data/*.db*' 'backend/data/logs' '*.log' 2>&1 |
    Where-Object { $_ -and ($_ -notmatch 'warning:') })
Write-Check 'runtime artifacts clean in git status' ($dirtyRuntime.Count -eq 0) $(if ($dirtyRuntime.Count) { ($dirtyRuntime -join "`n") } else { 'OK' }) $true

# Quant unit scripts
Write-Host ""
Write-Host '--- quant scripts ---' -ForegroundColor Cyan
$fe = Join-Path $Root 'frontend'
if ((Test-Path (Join-Path $fe 'package.json')) -and (Get-Command npm -ErrorAction SilentlyContinue)) {
    Push-Location $fe
    $npmOut = & npm run test:quant 2>&1 | Out-String
    Pop-Location
    Write-Check 'npm run test:quant' ($LASTEXITCODE -eq 0) ($npmOut.Trim() | Select-Object -Last 1)
} else {
    Write-Check 'npm run test:quant' $false 'frontend or npm missing' $true
}

Write-Host ""
Write-Host '=== summary ===' -ForegroundColor Cyan
if ($fail -eq 0 -and $warn -eq 0) {
    Write-Host "All checks passed. Next: Set-Location $Root; wails doctor; wails dev" -ForegroundColor Green
    exit 0
} elseif ($fail -eq 0) {
    Write-Host "Passed with $warn warning(s). Review warnings before production build." -ForegroundColor Yellow
    exit 0
} else {
    Write-Host "Failed: $fail  Warnings: $warn" -ForegroundColor Red
    Write-Host 'Install missing tools, open a NEW terminal, then re-run this script.' -ForegroundColor Yellow
    Write-Host "  1. Go $requiredGo+"
    Write-Host '  2. go install github.com/wailsapp/wails/v2/cmd/wails@latest'
    Write-Host '  3. Ensure %USERPROFILE%\go\bin and Go bin are on PATH'
    Write-Host '  4. wails doctor'
    exit 1
}

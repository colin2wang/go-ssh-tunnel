param(
    [string]$Version = "1.1.0",
    [string]$DeployDir = "build\bin"
)

$ErrorActionPreference = "Stop"

# Ensure GOPATH/bin is in PATH (for go-winres and other Go tools)
$gopathBin = "$env:GOPATH\bin"
if (-not $gopathBin -or $gopathBin -eq "\bin") { $gopathBin = "$env:USERPROFILE\go\bin" }
if ($env:PATH -notlike "*$gopathBin*") {
    $env:PATH = "$gopathBin;$env:PATH"
}

# Ensure pnpm is used for the frontend build (wails.json runs `pnpm install`)
if (-not (Get-Command pnpm -ErrorAction SilentlyContinue)) {
    Write-Host "pnpm not found! Install it first: npm install -g pnpm" -ForegroundColor Red
    exit 1
}

Write-Host "=== SSH Tunnel Builder ===" -ForegroundColor Cyan
Write-Host ""

# 1. Create directories
Write-Host "[1/3] Creating directories..." -ForegroundColor Yellow
if (-not (Test-Path $DeployDir)) {
    New-Item -ItemType Directory -Path $DeployDir -Force | Out-Null
    Write-Host "  Created: $DeployDir"
}

# 2. Build frontend + binary with Wails (output stays in build\bin)
Write-Host "[2/3] Building with wails..." -ForegroundColor Yellow
$timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
$ldflags = "-s -w -H windowsgui -X 'main.version=$Version' -X 'main.buildTime=$timestamp'"

wails build -ldflags $ldflags

if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed!" -ForegroundColor Red
    exit 1
}

# 3. Create config files next to the binary
Write-Host "[3/3] Creating config files..." -ForegroundColor Yellow

# Copy config.yaml: use existing if present, otherwise use main config
if (Test-Path "$DeployDir\config.yaml") {
    Write-Host "  Skipped: config.yaml (already exists)"
} elseif (Test-Path "configs\config.yaml") {
    Copy-Item "configs\config.yaml" "$DeployDir\config.yaml" -Force
    Write-Host "  Created: config.yaml (from configs/config.yaml)"
} else {
    Copy-Item "configs\config-example.yaml" "$DeployDir\config.yaml" -Force
    Write-Host "  Created: config.yaml (from example)"
}

# Create password.txt placeholder (only if not exists)
if (-not (Test-Path "$DeployDir\password.txt")) {
    "# Put your SSH password here (plain text)" | Out-File -FilePath "$DeployDir\password.txt" -Encoding ascii -Force
    Write-Host "  Created: password.txt (placeholder)"
} else {
    Write-Host "  Skipped: password.txt (already exists)"
}

# Summary
Write-Host ""
Write-Host "Build complete!" -ForegroundColor Green
Write-Host ""

$size = (Get-Item "$DeployDir\ssh-tunnel.exe").Length / 1MB
Write-Host "Deploy package: $DeployDir\" -ForegroundColor Cyan
Write-Host "  ssh-tunnel.exe  ($([math]::Round($size, 1)) MB)"
Write-Host "  config.yaml"
Write-Host "  password.txt (edit this!)"
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "  1. Edit $DeployDir\password.txt with your SSH password"
Write-Host "  2. Edit $DeployDir\config.yaml if needed"
Write-Host "  3. Run $DeployDir\ssh-tunnel.exe"

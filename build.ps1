param(
    [string]$Version = "1.0.9",
    [string]$Output = "dist\ssh-tunnel.exe",
    [string]$DeployDir = "dist"
)

$ErrorActionPreference = "Stop"

Write-Host "=== SSH Tunnel Builder ===" -ForegroundColor Cyan
Write-Host ""

# 1. Create directories
Write-Host "[1/5] Creating directories..." -ForegroundColor Yellow
if (-not (Test-Path $DeployDir)) {
    New-Item -ItemType Directory -Path $DeployDir -Force | Out-Null
    Write-Host "  Created: $DeployDir"
}

# 2. Generate Windows resource (icon)
Write-Host "[2/5] Generating Windows resource..." -ForegroundColor Yellow
Push-Location "cmd\ssh-tunnel"
go-winres simply --icon "../../transfor.ico" --manifest gui --file-description "SSH Tunnel Manager" --product-name "SSH Tunnel Manager" --arch amd64
Pop-Location

if ($LASTEXITCODE -ne 0) {
    Write-Host "Resource generation failed!" -ForegroundColor Red
    exit 1
}

# 3. Build binary
Write-Host "[3/5] Building binary..." -ForegroundColor Yellow
$timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
$ldflags = "-s -w -H windowsgui -X 'main.version=$Version' -X 'main.buildTime=$timestamp'"

go build -ldflags $ldflags -o $Output ./cmd/ssh-tunnel

if ($LASTEXITCODE -ne 0) {
    Write-Host "Build failed!" -ForegroundColor Red
    exit 1
}

# 4. Create config files
Write-Host "[4/5] Creating config files..." -ForegroundColor Yellow

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
Write-Host "[5/5] Build complete!" -ForegroundColor Green
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

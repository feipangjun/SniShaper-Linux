$ErrorActionPreference = "Stop"

$APP_NAME = "snishaper"
$BUILD_DIR = "build"
$WEB_DIR = "web"

Write-Host "=== SniShaper-Linux Build Script ===" -ForegroundColor Cyan

# Step 1: Build web frontend
Write-Host "[1/3] Building web frontend..." -ForegroundColor Yellow
Push-Location $WEB_DIR
npm install
npm run build
Pop-Location
Write-Host "[1/3] Web frontend build complete" -ForegroundColor Green

# Step 2: Cross-compile Go binary for Linux amd64
Write-Host "[2/3] Cross-compiling Go binary (linux/amd6⁴)..." -ForegroundColor Yellow
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -ldflags="-s -w" -o $APP_NAME .
if (-not $?) { throw "Go build failed" }
Write-Host "[2/3] Cross-compile complete: $APP_NAME" -ForegroundColor Green

# Step 3: Assemble build directory
Write-Host "[3/3] Assembling build directory..." -ForegroundColor Yellow
if (Test-Path $BUILD_DIR) {
	$retry = 0
	do {
		Start-Sleep -Milliseconds 200
		try { Remove-Item -Recurse -Force $BUILD_DIR -ErrorAction Stop; break }
		catch { $retry++ }
	} while ($retry -lt 5)
}
New-Item -ItemType Directory -Path "$BUILD_DIR\rules" -Force | Out-Null
New-Item -ItemType Directory -Path "$BUILD_DIR\config" -Force | Out-Null
New-Item -ItemType Directory -Path "$BUILD_DIR\core\mihomo" -Force | Out-Null

Move-Item -Force "$APP_NAME" "$BUILD_DIR\$APP_NAME"
Copy-Item -Force "rules\config.json" "$BUILD_DIR\rules\"
Copy-Item -Force "config\settings.json" "$BUILD_DIR\config\"
Copy-Item -Force "core\mihomo\mihomo" "$BUILD_DIR\core\mihomo\"
Copy-Item -Force "README.md" "$BUILD_DIR\"
Copy-Item -Force "README_EN.md" "$BUILD_DIR\"
Copy-Item -Force "LICENSE" "$BUILD_DIR\"
Write-Host "[3/3] Build assembly complete" -ForegroundColor Green

Write-Host ""
Write-Host ""
Write-Host "Usage:" -ForegroundColor Yellow
Write-Host "  Start proxy + web:  sudo ./$APP_NAME start    (web at http://127.0.0.1:5173)"
Write-Host "  Start proxy only:   sudo ./$APP_NAME start --no-web"
Write-Host "  Web only:           sudo ./$APP_NAME web"
Write-Host "  TUN mode:           sudo ./$APP_NAME tun start"
Write-Host ""
Write-Host "=== Build Complete ===" -ForegroundColor Cyan
Write-Host "Output: $BUILD_DIR\"
Write-Host "Binary: $BUILD_DIR\$APP_NAME"
Write-Host "Rules:  $BUILD_DIR\rules\"
Write-Host "Config: $BUILD_DIR\config\"
Write-Host "Mihomo: $BUILD_DIR\core\mihomo\"

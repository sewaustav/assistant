# Сборка assistant.exe на Windows.
# CGO и MinGW не нужны: микрофон захватывается через winmm (чистый Go).
# Запуск: powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

if (-not (Get-Command "go" -ErrorAction SilentlyContinue)) {
    Write-Host "Go не найден: https://go.dev/dl/" -ForegroundColor Red
    exit 1
}

$Version = "dev"
try {
    $Version = (git describe --tags --always --dirty 2>$null)
    if (-not $Version) { $Version = "dev" }
} catch { }

New-Item -ItemType Directory -Force -Path (Join-Path $Root "bin") | Out-Null

$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"

Write-Host ">>> Сборка assistant $Version..." -ForegroundColor Cyan
go build -trimpath -ldflags "-X main.version=$Version" -o (Join-Path $Root "bin\assistant.exe") .\cmd\assistant_app
if ($LASTEXITCODE -ne 0) {
    Write-Host "Сборка не удалась." -ForegroundColor Red
    exit $LASTEXITCODE
}

Write-Host ">>> Готово: .\bin\assistant.exe" -ForegroundColor Green

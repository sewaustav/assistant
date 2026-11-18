# Запуск assistant.exe на Windows.
# Запуск: powershell -ExecutionPolicy Bypass -File .\scripts\run-windows.ps1

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
$Exe = Join-Path $Root "bin\assistant.exe"

if (-not (Test-Path $Exe)) {
    Write-Host "Сначала собери: powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1" -ForegroundColor Red
    exit 1
}

Set-Location $Root
& $Exe

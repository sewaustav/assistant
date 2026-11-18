# Установка зависимостей ассистента на Windows.
# Запуск: powershell -ExecutionPolicy Bypass -File .\scripts\install-windows.ps1

$ErrorActionPreference = "Stop"

$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

function Require-Command($Name, $Hint) {
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        Write-Host "Не найден $Name." -ForegroundColor Red
        Write-Host $Hint
        exit 1
    }
}

Write-Host ">>> Проверка Go..." -ForegroundColor Cyan
Require-Command "go" "Установи Go: https://go.dev/dl/"

Write-Host ">>> Проверка Rust/cargo..." -ForegroundColor Cyan
Require-Command "cargo" "Установи Rust: https://rustup.rs/"

Write-Host ">>> Проверка Python..." -ForegroundColor Cyan
Require-Command "python" "Установи Python 3.10+: https://www.python.org/downloads/"

# cargo bin в PATH.
$CargoBin = Join-Path $env:USERPROFILE ".cargo\bin"
if ($env:Path -notlike "*$CargoBin*") {
    $env:Path = "$CargoBin;$env:Path"
    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($UserPath -notlike "*$CargoBin*") {
        [Environment]::SetEnvironmentVariable("Path", "$CargoBin;$UserPath", "User")
        Write-Host ">>> $CargoBin добавлен в PATH пользователя."
    }
}

Write-Host ">>> Установка gigastt..." -ForegroundColor Cyan
if (-not (Get-Command "gigastt" -ErrorAction SilentlyContinue)) {
    cargo install gigastt
} else {
    Write-Host ">>> gigastt уже установлен."
}

$Venv = Join-Path $Root "tts-server\.venv"
$Python = Join-Path $Venv "Scripts\python.exe"

if (-not (Test-Path $Venv)) {
    Write-Host ">>> Создаю venv..." -ForegroundColor Cyan
    python -m venv $Venv
}

Write-Host ">>> Установка Python-зависимостей (torch и др. — это долго)..." -ForegroundColor Cyan
& $Python -m pip install --upgrade pip
& $Python -m pip install -r (Join-Path $Root "tts-server\requirements.txt")

Write-Host ">>> Загрузка Go-зависимостей..." -ForegroundColor Cyan
go mod tidy
go mod download

Write-Host ">>> Готово. Теперь: powershell -ExecutionPolicy Bypass -File .\scripts\build-windows.ps1" -ForegroundColor Green

@echo off
chcp 65001 >nul
cd /d "%~dp0.."

echo ============================================
echo  DriftBottle - Build for Windows + Linux (amd64)
echo ============================================

echo [1/6] Build frontend ...
pushd frontend
call npm install || goto fail
call npm run build || goto fail
popd

echo [2/6] Copy frontend dist to embed dir ...
if exist backend\web\dist rmdir /s /q backend\web\dist
mkdir backend\web\dist
xcopy /e /i /y "frontend\dist\*" "backend\web\dist\" >nul || goto fail

for /f %%i in ('powershell -NoProfile -Command "Get-Date -Format yyMMdd_HHmmss"') do set "VER=%%i"
if "%VER%"=="" goto fail
set "VERDIR=dist\%VER%"

echo [3/6] Compile backend for Windows (GOOS=windows GOARCH=amd64) ...
mkdir "%VERDIR%"
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
pushd backend
go build -trimpath -ldflags "-s -w" -o "..\%VERDIR%\driftbottle.exe" . || goto fail
popd

echo [4/6] Compile backend for Linux (GOOS=linux GOARCH=amd64) ...
set GOOS=linux
pushd backend
go build -trimpath -ldflags "-s -w" -o "..\%VERDIR%\driftbottle" . || goto fail
popd

echo [5/6] Bundle manage.sh ...
copy /y "scripts\manage.sh" "%VERDIR%\manage.sh" >nul || goto fail

echo [6/6] Done. Output: %VERDIR%
echo   - driftbottle.exe (Windows)
echo   - driftbottle (Linux)
echo   - manage.sh
exit /b 0

:fail
echo Build failed.
exit /b 1

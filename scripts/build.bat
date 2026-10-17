@echo off
chcp 65001 >nul
cd /d "%~dp0.."

echo ============================================
echo  DriftBottle - Build for Linux (amd64)
echo ============================================

echo [1/5] Build frontend ...
pushd frontend
call npm install || goto fail
call npm run build || goto fail
popd

echo [2/5] Copy frontend dist to embed dir ...
if exist backend\web\dist rmdir /s /q backend\web\dist
mkdir backend\web\dist
xcopy /e /i /y "frontend\dist\*" "backend\web\dist\" >nul || goto fail

for /f %%i in ('powershell -NoProfile -Command "Get-Date -Format yyMMdd_HHmmss"') do set "VER=%%i"
if "%VER%"=="" goto fail
set "VERDIR=dist\%VER%"

echo [3/5] Compile backend (GOOS=linux GOARCH=amd64) ...
mkdir "%VERDIR%"
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64
pushd backend
go build -trimpath -ldflags "-s -w" -o "..\%VERDIR%\driftbottle" . || goto fail
popd

echo [4/5] Bundle manage.sh ...
copy /y "scripts\manage.sh" "%VERDIR%\manage.sh" >nul || goto fail

echo [5/5] Done. Output: %VERDIR%
echo   - driftbottle
echo   - manage.sh
exit /b 0

:fail
echo Build failed.
exit /b 1

@echo off
setlocal
cd /d "%~dp0"
if "%1"=="" (set TARGET=up) else (set TARGET=%1)

if /I "%TARGET%"=="up" (
  docker compose up --build -d
  docker compose ps
  echo App: http://localhost:8080
  goto :eof
)
if /I "%TARGET%"=="down" (
  docker compose down
  goto :eof
)
if /I "%TARGET%"=="logs" (
  docker compose logs -f api
  goto :eof
)

echo Usage: make.cmd [up^|down^|logs]
exit /b 1

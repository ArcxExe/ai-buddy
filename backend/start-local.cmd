@echo off
setlocal

where go >nul 2>&1
if errorlevel 1 (
  echo Go was not found in PATH. Install Go 1.24+ and reopen your terminal.
  exit /b 1
)

rem Only this cmd process and its children receive these demo settings.
set "APP_ENV=development"
set "BACKEND_PORT=18080"
if not "%~1"=="" set "BACKEND_PORT=%~1"
set "STORAGE_MODE=memory"
set "POSTGRES_DSN="
set "REDIS_ADDR="
set "REDIS_PASSWORD="
set "OPENROUTER_API_KEY="
set "LLM_PROVIDER=smart_fallback"
set "KNOWLEDGE_PATH="
set "FF_ENABLE_MCP_ACTIONS=true"
set "FF_ENABLE_RBAC_STRICT=true"
set "FF_ENABLE_CLICKSTREAM_ANALYTICS=true"
set "FF_ENABLE_PROACTIVE_CRON_HINTS=true"
set "CORS_ALLOWED_ORIGINS=http://127.0.0.1:5173,http://localhost:5173"

rem Check availability without stopping the process that owns a busy port.
powershell.exe -NoProfile -Command "$port=0; if (-not [int]::TryParse($env:BACKEND_PORT,[ref]$port) -or $port -lt 1 -or $port -gt 65535) { [Console]::Error.WriteLine('Invalid backend port. Use 1-65535.'); exit 1 }; $listeners=[System.Net.NetworkInformation.IPGlobalProperties]::GetIPGlobalProperties().GetActiveTcpListeners(); if ($listeners.Port -contains $port) { [Console]::Error.WriteLine('Backend port '+$port+' is unavailable. Choose another port; no existing process was stopped.'); exit 1 }"
if errorlevel 1 exit /b 1

pushd "%~dp0"
if errorlevel 1 exit /b 1
echo Starting isolated AI-buddy demo on port %BACKEND_PORT%. Press Ctrl+C to stop.
go run ./cmd/server
set "serverExitCode=%ERRORLEVEL%"
popd
exit /b %serverExitCode%

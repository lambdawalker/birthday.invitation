@echo off
setlocal
where go >nul 2>nul
if errorlevel 1 (
  echo Go is required. Install it from https://go.dev/dl/ and reopen your terminal.
  exit /b 1
)
pushd "%~dp0"
if errorlevel 1 exit /b 1
go run ./cmd/deploy %*
set "RESULT=%ERRORLEVEL%"
popd
exit /b %RESULT%

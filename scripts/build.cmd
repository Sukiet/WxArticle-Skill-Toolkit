@echo off
setlocal

set "ROOT=%~dp0.."
for %%I in ("%ROOT%") do set "ROOT=%%~fI"
set "OUTPUT_DIR=%ROOT%\debug"
set "BINARY_PATH=%OUTPUT_DIR%\skill-tool.exe"

if not exist "%OUTPUT_DIR%" (
  mkdir "%OUTPUT_DIR%"
)

set "CGO_ENABLED=0"

pushd "%ROOT%"
go build -trimpath -ldflags="-s -w" -o "%BINARY_PATH%" .
set "EXIT_CODE=%ERRORLEVEL%"
popd

if not "%EXIT_CODE%"=="0" (
  echo Build failed with exit code %EXIT_CODE%.
  pause
  exit /b %EXIT_CODE%
)

echo.
echo Build succeeded.
echo Output: %BINARY_PATH%
pause

@echo off
REM Builds a release binary into bin\windows.
setlocal
cd /d "%~dp0"

if not exist bin\windows mkdir bin\windows

REM -H windowsgui keeps a console window from opening alongside the application.
go build -trimpath -ldflags "-s -w -H windowsgui" -o bin\windows\pdx-portrait-creator.exe .\cmd\pdx-portrait-creator
if errorlevel 1 exit /b 1

echo built bin\windows\pdx-portrait-creator.exe

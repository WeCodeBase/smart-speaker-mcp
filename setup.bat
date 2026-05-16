@echo off
REM ─────────────────────────────────────────────────────────────────────────────
REM smart-speaker-mcp — Windows installer (legacy cmd.exe wrapper)
REM
REM Delegates to setup.ps1, which does all the real work. We use this thin
REM wrapper so users who double-click a .bat file from Explorer get the same
REM installer as PowerShell users — without us having to reimplement JSON
REM merging in batch syntax (cmd.exe can't parse JSON).
REM
REM Usage:
REM   setup.bat                  -- latest release
REM   setup.bat v3.2.0           -- specific release
REM ─────────────────────────────────────────────────────────────────────────────

setlocal

set "SCRIPT_DIR=%~dp0"
set "PS1_SCRIPT=%SCRIPT_DIR%setup.ps1"

if not exist "%PS1_SCRIPT%" (
    echo ERROR: Cannot find setup.ps1 next to setup.bat
    echo Expected at: %PS1_SCRIPT%
    pause
    exit /b 1
)

REM Forward an optional version argument as -Version
set "PS_ARGS="
if not "%~1"=="" set "PS_ARGS=-Version %~1"

REM Bypass execution policy for this single invocation only
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%PS1_SCRIPT%" %PS_ARGS%
set "EXITCODE=%ERRORLEVEL%"

echo.
if %EXITCODE% NEQ 0 (
    echo Setup failed with exit code %EXITCODE%.
) else (
    echo Setup finished.
)

REM Pause so users who double-clicked can read the output before the window closes
pause
exit /b %EXITCODE%

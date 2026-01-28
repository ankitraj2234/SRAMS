@echo off
REM SRAMS PostgreSQL Stop Service Script
REM Gracefully stops PostgreSQL server

setlocal

set "INSTALL_DIR=%~dp0.."
set "PG_DIR=%INSTALL_DIR%\pgsql"
set "PG_DATA=%INSTALL_DIR%\data\postgres"

echo Stopping PostgreSQL server...

REM Check if running
"%PG_DIR%\bin\pg_isready.exe" -h localhost -p 5432 >nul 2>&1
if errorlevel 1 (
    echo PostgreSQL is not running.
    exit /b 0
)

REM Stop PostgreSQL gracefully
"%PG_DIR%\bin\pg_ctl.exe" -D "%PG_DATA%" stop -m fast

if errorlevel 1 (
    echo WARNING: Graceful shutdown failed, forcing...
    "%PG_DIR%\bin\pg_ctl.exe" -D "%PG_DATA%" stop -m immediate
)

echo PostgreSQL server stopped.
exit /b 0

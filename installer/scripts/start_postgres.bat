@echo off
REM SRAMS PostgreSQL Start Service Script
REM Starts PostgreSQL as a background process

setlocal

set "INSTALL_DIR=%~dp0.."
set "PG_DIR=%INSTALL_DIR%\pgsql"
set "PG_DATA=%INSTALL_DIR%\data\postgres"
set "PG_LOG=%INSTALL_DIR%\logs\postgres.log"

REM Check if already running
"%PG_DIR%\bin\pg_isready.exe" -h localhost -p 5432 >nul 2>&1
if not errorlevel 1 (
    echo PostgreSQL is already running.
    exit /b 0
)

echo Starting PostgreSQL server...

REM Start PostgreSQL
"%PG_DIR%\bin\pg_ctl.exe" -D "%PG_DATA%" -l "%PG_LOG%" start

if errorlevel 1 (
    echo ERROR: Failed to start PostgreSQL server
    exit /b 1
)

REM Wait for ready
:wait_loop
"%PG_DIR%\bin\pg_isready.exe" -h localhost -p 5432 >nul 2>&1
if errorlevel 1 (
    timeout /t 1 /nobreak >nul
    goto wait_loop
)

echo PostgreSQL server started successfully.
exit /b 0

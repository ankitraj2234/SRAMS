@echo off
REM SRAMS PostgreSQL Setup Script
REM This script initializes PostgreSQL during installation

setlocal enabledelayedexpansion

echo ================================================
echo SRAMS PostgreSQL Database Setup
echo ================================================

set "INSTALL_DIR=%~dp0"
set "PG_DIR=%INSTALL_DIR%pgsql"
set "PG_DATA=%INSTALL_DIR%data\postgres"
set "PG_LOG=%INSTALL_DIR%logs\postgres.log"

REM Create directories
if not exist "%PG_DATA%" mkdir "%PG_DATA%"
if not exist "%INSTALL_DIR%logs" mkdir "%INSTALL_DIR%logs"

REM Check if PostgreSQL data directory is initialized
if not exist "%PG_DATA%\PG_VERSION" (
    echo Initializing PostgreSQL database cluster...
    
    REM Initialize database cluster
    "%PG_DIR%\bin\initdb.exe" -D "%PG_DATA%" -U postgres -E UTF8 --locale=C
    
    if errorlevel 1 (
        echo ERROR: Failed to initialize PostgreSQL database
        exit /b 1
    )
    
    echo PostgreSQL database cluster initialized successfully.
)

REM Configure PostgreSQL for local connections only
echo Configuring PostgreSQL for SRAMS...

REM Update pg_hba.conf for local connections
(
    echo # SRAMS PostgreSQL Authentication
    echo # DO NOT EDIT - Managed by SRAMS installer
    echo local   all             all                                     trust
    echo host    all             all             127.0.0.1/32            md5
    echo host    all             all             ::1/128                 md5
) > "%PG_DATA%\pg_hba.conf"

REM Update postgresql.conf for SRAMS
(
    echo # SRAMS PostgreSQL Configuration
    echo listen_addresses = 'localhost'
    echo port = 5432
    echo max_connections = 50
    echo shared_buffers = 128MB
    echo effective_cache_size = 256MB
    echo work_mem = 4MB
    echo maintenance_work_mem = 64MB
    echo logging_collector = on
    echo log_directory = 'log'
    echo log_filename = 'postgresql-%%Y-%%m-%%d.log'
    echo log_statement = 'ddl'
) > "%PG_DATA%\postgresql.conf"

echo PostgreSQL configuration complete.
echo.
echo Starting PostgreSQL server...

REM Start PostgreSQL server
"%PG_DIR%\bin\pg_ctl.exe" -D "%PG_DATA%" -l "%PG_LOG%" start

if errorlevel 1 (
    echo ERROR: Failed to start PostgreSQL server
    exit /b 1
)

REM Wait for server to be ready
echo Waiting for PostgreSQL to be ready...
:wait_loop
"%PG_DIR%\bin\pg_isready.exe" -h localhost -p 5432 >nul 2>&1
if errorlevel 1 (
    timeout /t 1 /nobreak >nul
    goto wait_loop
)

echo PostgreSQL server is running.
echo.

REM Create SRAMS database and users
echo Creating SRAMS database and users...

"%PG_DIR%\bin\psql.exe" -h localhost -U postgres -c "CREATE DATABASE srams;" 2>nul
"%PG_DIR%\bin\psql.exe" -h localhost -U postgres -c "CREATE USER srams_app WITH PASSWORD '%SRAMS_DB_PASSWORD%';" 2>nul
"%PG_DIR%\bin\psql.exe" -h localhost -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE srams TO srams_app;" 2>nul
"%PG_DIR%\bin\psql.exe" -h localhost -U postgres -c "ALTER DATABASE srams OWNER TO srams_app;" 2>nul

echo SRAMS database created.
echo.

REM Run migrations
echo Running database migrations...

set "MIGRATIONS_DIR=%INSTALL_DIR%migrations"

if exist "%MIGRATIONS_DIR%" (
    for %%f in ("%MIGRATIONS_DIR%\*.sql") do (
        echo Running migration: %%~nxf
        "%PG_DIR%\bin\psql.exe" -h localhost -U postgres -d srams -f "%%f"
        if errorlevel 1 (
            echo WARNING: Migration %%~nxf may have had issues
        )
    )
)

echo Migrations complete.
echo.

echo ================================================
echo PostgreSQL setup complete!
echo.
echo Database: srams
echo Host: localhost
echo Port: 5432
echo User: srams_app
echo ================================================

exit /b 0

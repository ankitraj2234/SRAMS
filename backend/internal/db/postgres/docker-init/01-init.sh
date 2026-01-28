#!/bin/bash
# SRAMS PostgreSQL Initialization Script
# Runs automatically when Docker container starts for the first time

set -e

echo "=== SRAMS PostgreSQL Initialization ==="

# Create application user with limited privileges
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Create application role (used by Go backend)
    DO \$\$
    BEGIN
        IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'srams_app') THEN
            CREATE ROLE srams_app WITH LOGIN PASSWORD 'srams_app_2026' NOSUPERUSER NOCREATEDB NOCREATEROLE;
        END IF;
    END
    \$\$;

    -- Create read-only role (for reporting)
    DO \$\$
    BEGIN
        IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'srams_readonly') THEN
            CREATE ROLE srams_readonly WITH LOGIN PASSWORD 'srams_readonly_2026' NOSUPERUSER NOCREATEDB NOCREATEROLE;
        END IF;
    END
    \$\$;

    -- Grant connect privileges
    GRANT CONNECT ON DATABASE srams TO srams_app;
    GRANT CONNECT ON DATABASE srams TO srams_readonly;

EOSQL

echo "=== Users created successfully ==="
echo "=== Now running migration files ==="

# Run migration files in order
for f in /docker-entrypoint-initdb.d/migrations/*.sql; do
    if [ -f "$f" ]; then
        echo "Running migration: $f"
        psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -f "$f"
    fi
done

echo "=== SRAMS PostgreSQL Initialization Complete ==="

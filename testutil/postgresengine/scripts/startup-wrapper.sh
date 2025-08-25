#!/bin/bash
# PostgreSQL Startup Wrapper with Catalog Tuning Application
# 
# This script ensures that all catalog-stored tuning settings are applied
# on every container startup, preventing persistence issues with volume mounts.
# 
# Flow:
# 1. Start PostgreSQL in background
# 2. Wait for it to be ready
# 3. Apply catalog tuning settings from apply-catalog-tuning.sql
# 4. Keep PostgreSQL running in foreground

set -e

echo "🚀 Starting PostgreSQL with automatic catalog tuning application..."

# Start PostgreSQL in background with the same parameters as original command
echo "📦 Starting PostgreSQL server..."
docker-entrypoint.sh postgres \
    -c config_file=/etc/postgresql/postgresql.conf \
    -c hba_file=/etc/postgresql/pg_hba.conf &

# Store PostgreSQL process ID
PG_PID=$!

# Function to check if PostgreSQL is ready
wait_for_postgres() {
    local max_attempts=60  # 60 seconds timeout
    local attempt=0
    
    echo "⏳ Waiting for PostgreSQL to be ready..."
    
    while [ $attempt -lt $max_attempts ]; do
        if PGPASSWORD=$POSTGRES_PASSWORD pg_isready -U test -d eventstore -h localhost >/dev/null 2>&1; then
            echo "✅ PostgreSQL is ready!"
            return 0
        fi
        
        # Check if PostgreSQL process is still running
        if ! kill -0 $PG_PID 2>/dev/null; then
            echo "❌ PostgreSQL process died during startup!"
            exit 1
        fi
        
        echo "   Attempt $((attempt + 1))/$max_attempts - PostgreSQL not ready yet..."
        sleep 1
        attempt=$((attempt + 1))
    done
    
    echo "❌ Timeout: PostgreSQL did not become ready within $max_attempts seconds"
    exit 1
}

# Wait for PostgreSQL to be ready
wait_for_postgres

# Apply catalog tuning settings
echo "🔧 Applying catalog tuning settings..."
if PGPASSWORD=$POSTGRES_PASSWORD psql -U test -d eventstore -h localhost -f /tuning/apply-catalog-tuning.sql; then
    echo "✅ Catalog tuning settings applied successfully!"
else
    echo "❌ Failed to apply catalog tuning settings!"
    echo "   PostgreSQL will continue running, but settings may not be optimal."
fi

echo "🎯 PostgreSQL startup complete with tuning applied!"
echo "📊 Container is ready for connections on port 5432"

# Keep PostgreSQL running in foreground
# This ensures the container doesn't exit and Docker can properly handle signals
wait $PG_PID
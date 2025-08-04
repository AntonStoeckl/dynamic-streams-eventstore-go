#!/bin/bash

# Script to restart PostgreSQL benchmark container with new performance configuration
# This clears the existing data volume to ensure the new configuration is applied

echo "🔄 Restarting PostgreSQL benchmark container with performance optimizations..."

# Stop the container
echo "Stopping postgres_benchmark container..."
docker compose stop postgres_benchmark

# Remove the container to ensure clean restart
echo "Removing postgres_benchmark container..."
docker compose rm -f postgres_benchmark

# Remove the persistent volume to force reinitialization with new config
echo "⚠️  Removing pgdata volume to apply new configuration..."
docker volume rm des_pgdata 2>/dev/null || echo "Volume didn't exist or was already removed"

# Start the container with new configuration
echo "Starting postgres_benchmark with performance optimizations..."
docker compose up -d postgres_benchmark

# Wait for PostgreSQL to be ready
echo "⏳ Waiting for PostgreSQL to be ready..."
for i in {1..30}; do
    if docker compose exec postgres_benchmark pg_isready -U test -d eventstore >/dev/null 2>&1; then
        echo "✅ PostgreSQL is ready!"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "❌ PostgreSQL failed to start within 30 seconds"
        exit 1
    fi
    sleep 1
done

# Verify the performance configuration is loaded
echo "🔍 Verifying performance configuration..."
echo "Checking key autovacuum settings:"
docker compose exec postgres_benchmark psql -U test -d eventstore -c "
SELECT name, setting, unit, short_desc 
FROM pg_settings 
WHERE name IN (
    'autovacuum_naptime',
    'autovacuum_analyze_threshold', 
    'autovacuum_analyze_scale_factor',
    'default_statistics_target',
    'max_wal_size'
) 
ORDER BY name;"

# Check if events table tuning was applied
echo ""
echo "Checking events table specific settings:"
docker compose exec postgres_benchmark psql -U test -d eventstore -c "
SELECT unnest(reloptions) as setting 
FROM pg_class 
WHERE relname = 'events';"

# Import fixtures if needed
echo ""
echo "🔄 Importing fixture data..."
make -C . fixtures-import-benchmark 2>/dev/null || echo "Fixtures import skipped (make target not available)"

echo ""
echo "✅ PostgreSQL benchmark container restarted with performance optimizations!"
echo "📊 Monitor performance with: SELECT * FROM check_events_table_health();"
echo "🔍 View autovacuum stats: SELECT * FROM v_events_autovacuum_stats;"
#!/bin/bash
# PostgreSQL 17.5 Replica Setup Script for EventStore Benchmark Environment
# This script initializes a streaming replica from the benchmark master

set -e  # Exit on any error
set -u  # Exit on undefined variables

# Configuration variables
MASTER_HOST="${POSTGRES_MASTER_HOST:-postgres_benchmark_master}"
MASTER_PORT="${POSTGRES_MASTER_PORT:-5432}"
MASTER_USER="${POSTGRES_MASTER_USER:-test}"
MASTER_PASSWORD="${POSTGRES_MASTER_PASSWORD:-test}"
MASTER_DB="${POSTGRES_MASTER_DB:-eventstore}"
REPLICATION_USER="${POSTGRES_REPLICATION_USER:-replicator}"
REPLICATION_PASSWORD="${POSTGRES_REPLICATION_PASSWORD:-replicator_password}"
REPLICA_SLOT="${POSTGRES_REPLICA_SLOT:-}"

echo "=========================================="
echo "PostgreSQL 17.5 Replica Setup Starting"
echo "Master Host: $MASTER_HOST"
echo "Master Port: $MASTER_PORT"
echo "Replication User: $REPLICATION_USER"
echo "Replica Slot: ${REPLICA_SLOT:-auto-assigned}"
echo "=========================================="

# Wait for master to be ready with timeout
echo "Waiting for master database to be ready..."
TIMEOUT=120
COUNTER=0
until PGPASSWORD="$MASTER_PASSWORD" pg_isready -h "$MASTER_HOST" -p "$MASTER_PORT" -U "$MASTER_USER" -d "$MASTER_DB" -q; do
  COUNTER=$((COUNTER + 1))
  if [ $COUNTER -gt $TIMEOUT ]; then
    echo "ERROR: Master database not ready after ${TIMEOUT} seconds"
    exit 1
  fi
  echo "Waiting for master database... (${COUNTER}/${TIMEOUT})"
  sleep 1
done

echo "✓ Master database is ready"

# Verify replication user exists and has proper permissions
echo "Verifying replication user permissions..."
if ! PGPASSWORD="$REPLICATION_PASSWORD" psql -h "$MASTER_HOST" -p "$MASTER_PORT" -U "$REPLICATION_USER" -d "$MASTER_DB" -c "SELECT 1;" > /dev/null 2>&1; then
  echo "ERROR: Cannot connect with replication user. Please ensure:"
  echo "  1. User 'replicator' exists on master"
  echo "  2. User has REPLICATION privileges"
  echo "  3. pg_hba.conf allows replication connections"
  exit 1
fi
echo "✓ Replication user verified"

# Stop PostgreSQL if running
echo "Stopping PostgreSQL service..."
if pg_ctl status -D "$PGDATA" > /dev/null 2>&1; then
  pg_ctl stop -D "$PGDATA" -m fast -w
  echo "✓ PostgreSQL stopped"
else
  echo "✓ PostgreSQL was not running"
fi

# Remove existing data directory contents (Docker volume scenario)
if [ -d "$PGDATA" ] && [ "$(ls -A "$PGDATA" 2>/dev/null)" ]; then
  echo "Removing existing data directory contents for fresh replica setup..."
  rm -rf "$PGDATA"/*
  echo "✓ Existing data directory cleared"
fi

# Ensure data directory exists with proper permissions
echo "Preparing data directory for pg_basebackup..."
mkdir -p "$PGDATA"
chown postgres:postgres "$PGDATA"
chmod 700 "$PGDATA"

# Create base backup from master using PostgreSQL 17.5 enhanced pg_basebackup
echo "Creating base backup from master..."
echo "This may take several minutes depending on database size..."

PGBASEBACKUP_OPTS="-h $MASTER_HOST -p $MASTER_PORT -U $REPLICATION_USER -D $PGDATA -P -W -R --write-recovery-conf"

# Add replication slot if specified
if [ -n "$REPLICA_SLOT" ]; then
  echo "Using replication slot: $REPLICA_SLOT"
  PGBASEBACKUP_OPTS="$PGBASEBACKUP_OPTS -S $REPLICA_SLOT"
fi

# Note: Compression only works with tar mode, not plain mode
# For Docker setup, we use plain mode for simplicity

# Execute base backup with proper error handling
if ! PGPASSWORD="$REPLICATION_PASSWORD" pg_basebackup $PGBASEBACKUP_OPTS; then
  echo "ERROR: pg_basebackup failed. Common issues:"
  echo "  1. Network connectivity to master"
  echo "  2. Replication user permissions"
  echo "  3. pg_hba.conf configuration"
  echo "  4. Available disk space"
  exit 1
fi

echo "✓ Base backup completed successfully"

# Set proper permissions
echo "Setting proper permissions..."
chown -R postgres:postgres "$PGDATA"
chmod 700 "$PGDATA"
find "$PGDATA" -type f -exec chmod 600 {} \;
find "$PGDATA" -type d -exec chmod 700 {} \;
echo "✓ Permissions set"

# Verify recovery configuration
echo "Verifying recovery configuration..."
if [ -f "$PGDATA/postgresql.auto.conf" ]; then
  echo "✓ Found postgresql.auto.conf"
  if grep -q "primary_conninfo" "$PGDATA/postgresql.auto.conf"; then
    echo "✓ Primary connection info configured"
  else
    echo "WARNING: primary_conninfo not found in postgresql.auto.conf"
  fi
else
  echo "ERROR: postgresql.auto.conf not found - recovery may not work"
  exit 1
fi

# Create standby.signal file if it doesn't exist (required for PostgreSQL 12+)
if [ ! -f "$PGDATA/standby.signal" ]; then
  echo "Creating standby.signal file..."
  touch "$PGDATA/standby.signal"
  chown postgres:postgres "$PGDATA/standby.signal"
  chmod 600 "$PGDATA/standby.signal"
  echo "✓ standby.signal created"
else
  echo "✓ standby.signal already exists"
fi

# Test replica startup (quick test)
echo "Testing replica startup..."
if pg_ctl start -D "$PGDATA" -w -t 30; then
  echo "✓ Replica started successfully"
  
  # Wait a moment for replication to initialize
  sleep 5
  
  # Test replication connection
  if psql -h localhost -p 5432 -U "$MASTER_USER" -d "$MASTER_DB" -c "SELECT pg_is_in_recovery();" | grep -q "t"; then
    echo "✓ Replica is in recovery mode (correct)"
  else
    echo "WARNING: Replica is not in recovery mode"
  fi
  
  # Check replication status
  echo "Checking replication status..."
  REPLICATION_STATUS=$(psql -h localhost -p 5432 -U "$MASTER_USER" -d "$MASTER_DB" -t -c "SELECT CASE WHEN pg_is_in_recovery() THEN 'REPLICA' ELSE 'MASTER' END;")
  echo "Status: $REPLICATION_STATUS"
  
  # Stop for now - Docker will manage the startup
  pg_ctl stop -D "$PGDATA" -m fast -w
  echo "✓ Test completed, replica stopped for Docker management"
else
  echo "ERROR: Replica failed to start"
  echo "Check logs in $PGDATA/log/ for details"
  exit 1
fi

echo "=========================================="
echo "PostgreSQL 17.5 Replica Setup Completed Successfully"
echo ""
echo "Next steps:"
echo "  1. Replica will be started by Docker Compose"
echo "  2. Monitor replication lag with: SELECT * FROM v_replication_status;"
echo "  3. Check replication health with: SELECT * FROM check_replication_health();"
echo ""
echo "Configuration details:"
echo "  - Data directory: $PGDATA"
echo "  - Master host: $MASTER_HOST:$MASTER_PORT"
echo "  - Replication slot: ${REPLICA_SLOT:-none}"
echo "  - Recovery mode: Enabled (standby.signal present)"
echo "=========================================="
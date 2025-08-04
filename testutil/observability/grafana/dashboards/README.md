# EventStore Grafana Dashboard  

This directory contains the **EventStore Performance Dashboard** - a simplified, focused dashboard for monitoring EventStore operations from a real application perspective.

## EventStore Performance Dashboard
**File**: `eventstore-simplified.json`

Clean 6-panel dashboard focused purely on **EventStore operations** without load generator confusion:

### 📊 **Core Panels**
1. **Append Operations/sec** - How many append operations per second  
2. **Query Operations/sec** - How many query operations per second
3. **Operation Success Rate (%)** - Percentage of successful operations (gauge with thresholds)
4. **Average Operation Duration** - Mean append/query duration in milliseconds  
5. **Concurrency Conflicts/sec** - Optimistic concurrency conflicts (expected in high-concurrency apps)
6. **Error Rate by Type** - Breakdown of error types (database_query, concurrency_conflict, etc.)

### 🎯 **Key Benefits**
- ✅ **Clear 1:1 mapping**: 250 req/sec load generator = ~250 total operations/sec
- ✅ **Real application perspective**: Shows what any production app would see  
- ✅ **No load generator noise**: Pure EventStore performance metrics
- ✅ **Immediate understanding**: Operations/sec, success rate, avg duration, conflicts
- ✅ **Simplified visualization**: 6 focused panels instead of complex percentiles

### 📈 **Core Metrics**
- `eventstore_append_duration_seconds_count` - Append operations count (for ops/sec)
- `eventstore_query_duration_seconds_count` - Query operations count (for ops/sec)  
- `eventstore_*_duration_seconds_sum/_count` - Average duration calculations
- `eventstore_concurrency_conflicts_total` - Concurrency conflict tracking
- `eventstore_database_errors_total` - Error breakdown by type

## Auto-Provisioning

Dashboard is automatically loaded when the observability stack starts via:
- Volume mount: `./grafana/dashboards:/var/lib/grafana/dashboards`
- Provisioning config: `./grafana/provisioning/dashboards/dashboards.yml`

## Usage

1. **Start Observability Stack**: 
   ```bash
   cd testutil/observability && docker compose up -d
   ```

2. **Start PostgreSQL Database**:
   ```bash
   cd testutil/postgresengine && docker compose up -d postgres_benchmark
   ```

3. **Run Load Generator** (simplified, no load generator metrics):
   ```bash
   cd example/demo/cmd/load-generator
   ./load-generator --observability-enabled=true --rate=250
   ```

4. **Access Dashboard**:
   - Grafana UI: http://localhost:3000 (admin/secretpw)
   - Navigate to "EventStore" folder → "EventStore Performance Dashboard"

## What You'll See

With load generator at 250 req/sec:
- **Total Operations/sec**: ~250 (sum of append + query panels)
- **Success Rate**: >95% (green gauge)
- **Average Duration**: <10ms for typical operations  
- **Concurrency Conflicts**: A few per second (normal)
- **Error Breakdown**: Mostly concurrency conflicts (expected)

## Dashboard Features

- **Real-time Updates**: 5-second refresh for live monitoring
- **Time Range**: Default 15-minute window with quick selectors (5m, 30m, 1h, 3h)
- **Color Coding**: Thresholds for performance indicators (green/yellow/red)
- **Focused Metrics**: Only essential EventStore performance data
- **Production-Ready**: Suitable for both development and production monitoring

Perfect for understanding EventStore performance without dashboard confusion!
# EventStore Grafana Dashboard  

This directory contains the **EventStore Performance Dashboard** - a comprehensive dashboard for monitoring EventStore operations with complete observability coverage including cancellation tracking.

## EventStore Performance Dashboard
**File**: `eventstore-dashboard.json`

Comprehensive 16-panel dashboard providing complete **EventStore operations** monitoring with full status tracking including cancellation and timeout detection:

### 📊 **Panel Overview**

#### **EventStore Core Operations** (Row 1)
1. **Successful EventStore.Append() Ops/sec** - Successful append operations per second
2. **Successful EventStore.Query() Ops/sec** - Successful query operations per second  
3. **EventStore.Append() Errors/sec** - Failed append operations per second
4. **EventStore.Query() Errors/sec** - Failed query operations per second

#### **SQL Operations** (Row 2)
5. **SQL Insert Ops/sec** - Database INSERT operations per second
6. **SQL Select Ops/sec** - Database SELECT operations per second
7. **Average EventStore.Append() Duration** - Mean append duration in milliseconds
8. **Average EventStore.Query() Duration** - Mean query duration in milliseconds

#### **Command/Query Handler Operations** (Row 3)
9. **Successful Command Operations/sec** - Successful command handler operations by type
10. **Command Errors/sec** - Failed command handler operations by type

#### **Advanced Operations** (Row 4)
11. **Concurrency Conflicts/sec** - Optimistic concurrency conflicts (expected in high-concurrency apps)
12. **Command Idempotent Operations/sec** - Idempotent operations that resulted in no state change

#### **🆕 Cancellation Tracking** (Row 5)
13. **Cancelled EventStore Operations/sec** - **NEW**: Context-cancelled Query and Append operations
14. **Cancelled Command/Query Operations/sec** - **NEW**: Context-cancelled Command/Query Handler operations

#### **🆕 Timeout Tracking** (Row 6)
15. **Timeout EventStore Operations/sec** - **NEW**: Context timeout Query and Append operations
16. **Timeout Command/Query Operations/sec** - **NEW**: Context timeout Command/Query Handler operations

### 🎯 **Key Benefits**
- ✅ **Complete Observability**: Full EventStore operation monitoring with success/error/cancellation/timeout tracking
- ✅ **Context Error Detection**: NEW - Separate tracking for context cancellation vs timeout errors
- ✅ **Real Application Perspective**: Shows what production applications see
- ✅ **Comprehensive Status Tracking**: Success, error, cancelled, timeout, and idempotent operation classification
- ✅ **Multi-Level Monitoring**: EventStore operations, SQL operations, and Command/Query handlers
- ✅ **Production-Ready**: File-based provisioning with persistent configuration

### 📈 **Metrics Coverage**

#### **EventStore Operations**
- `eventstore_append_duration_seconds_count{status="success|error|cancelled|timeout"}` - Append operation counts
- `eventstore_query_duration_seconds_count{status="success|error|cancelled|timeout"}` - Query operation counts  
- `eventstore_*_duration_seconds_sum/_count` - Average duration calculations
- `eventstore_query_cancelled_total` - **NEW**: Cancelled query operations
- `eventstore_append_cancelled_total` - **NEW**: Cancelled append operations
- `eventstore_query_timeout_total` - **NEW**: Timeout query operations
- `eventstore_append_timeout_total` - **NEW**: Timeout append operations
- `eventstore_concurrency_conflicts_total` - Optimistic concurrency conflicts

#### **Command/Query Handler Operations**  
- `commandhandler_handle_duration_seconds_count{status="success|error|cancelled|timeout"}` - Handler operation counts
- `commandhandler_idempotent_operations_total` - Idempotent operations (no state change)
- `commandhandler_cancelled_operations_total` - **NEW**: Cancelled handler operations
- `commandhandler_timeout_operations_total` - **NEW**: Timeout handler operations

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
   - Direct URL: http://localhost:3000/d/eventstore-main-dashboard/eventstore-dashboard

## What You'll See

With load generator at 250 req/sec:
- **Total Operations/sec**: ~250 (sum of successful append + query operations)
- **Success Rate**: >95% for both EventStore and Command Handler operations
- **Average Duration**: <10ms for typical operations  
- **Concurrency Conflicts**: A few per second (normal behavior)
- **Cancelled Operations**: Should be 0/sec under normal conditions (indicates client cancellation when >0)
- **Timeout Operations**: Should be 0/sec under normal conditions (indicates context deadline exceeded when >0)
- **Idempotent Operations**: Commands that resulted in no state change

## Dashboard Features

- **Real-time Updates**: 5-second refresh for live monitoring
- **Time Range**: Default 5-minute window with quick selectors (5m, 30m, 1h, 3h)
- **Comprehensive Coverage**: 16 panels covering all aspects of EventStore operations
- **Status Tracking**: Complete success/error/cancelled/timeout/idempotent operation classification
- **File-Based Provisioning**: Persistent across Docker restarts with version control
- **Production-Ready**: Suitable for both development and production monitoring

## 🆕 Context Error Tracking

The dashboard now includes **context cancellation and timeout detection** - critical observability features for production systems that distinguish between user cancellation vs system timeouts:

### **Context.Canceled vs Context.DeadlineExceeded**

**Context.Canceled** (User Cancellation):
- **Client Cancellation**: Application explicitly cancels context
- **Network Issues**: Connection drops during operations
- **System Shutdown**: Graceful shutdown cancels in-flight operations
- **User Actions**: User cancels long-running requests

**Context.DeadlineExceeded** (System Timeouts):
- **Context Timeouts**: Application sets short context deadlines
- **Database Timeouts**: Operations exceed configured limits
- **Load Balancer Timeouts**: Infrastructure-level request timeouts
- **System Performance**: Operations take longer than expected

### **Monitoring Benefits**
- **Detect System Stress**: High timeout rates indicate performance bottlenecks
- **Debug Client Issues**: High cancellation rates suggest aggressive client behavior
- **Capacity Planning**: Timeout patterns reveal system limits and tuning needs
- **Operational Health**: Distinguish real errors from context-related issues
- **Performance Tuning**: Separate tracking enables targeted optimizations

Perfect for comprehensive EventStore performance monitoring with complete operational visibility!
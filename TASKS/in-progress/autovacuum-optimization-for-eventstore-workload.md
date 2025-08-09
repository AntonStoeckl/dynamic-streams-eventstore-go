## Autovacuum Optimization for EventStore Workload
- **Started**: 2025-08-07
- **Priority**: High - Periodic timeout spikes and performance degradation with growing database
- **Objective**: Tune autovacuum for append-only EventStore workload to eliminate blocking and improve consistent performance

### Performance Issues Identified
- **Periodic Timeout Spikes**: Every couple of minutes on Grafana, not fixed interval
- **Append Operations Affected**: Timeouts almost exclusively on Append() operations (writes to master)
- **Performance Degradation**: 250+ req/s shows declining performance as database grows (447K events)
- **Autovacuum Blocking**: Write operations blocked during vacuum cycles

### EventStore-Specific Autovacuum Requirements
**Key Insight**: EventStore has unique characteristics requiring different autovacuum strategy:

1. **No DELETE Operations**: Pure append-only workload (except manual cleanup in tests)
2. **Minimal Dead Tuples**: Very few dead tuples generated during normal operations
3. **High INSERT Rate**: Continuous event appends create many new tuples
4. **Query Performance Critical**: ANALYZE needs to run frequently for JSONB query optimization
5. **Write Blocking Unacceptable**: Vacuum blocking Append() operations causes timeouts

### Proposed Autovacuum Tuning Strategy

#### Master Database (Write-Heavy)
**Minimize vacuum frequency, maximize ANALYZE frequency:**
```sql
-- Vacuum rarely (minimal dead tuples in append-only)
autovacuum_vacuum_scale_factor = 0.8    # Only vacuum at 80% dead tuples (vs current 10%)
autovacuum_naptime = 2min               # Check more frequently but vacuum rarely
autovacuum_vacuum_cost_delay = 2ms      # Faster vacuum when it does run (vs 10ms)
autovacuum_vacuum_cost_limit = 8000     # Higher throughput during vacuum (vs 2000)

-- Analyze frequently (critical for JSONB query planning)  
autovacuum_analyze_scale_factor = 0.02  # Analyze at 2% changed tuples (vs 5%)
```

#### Replica Database (Read-Heavy)
**Minimal vacuum, frequent ANALYZE for query optimization:**
```sql
-- Even less vacuum (read-only, no new dead tuples)
autovacuum_vacuum_scale_factor = 0.9    # Almost never vacuum
autovacuum_naptime = 10min              # Less frequent checks

-- More frequent ANALYZE (query performance critical)
autovacuum_analyze_scale_factor = 0.05  # Analyze at 5% changed tuples (vs 10%)
```

### Implementation Plan
1. **Update master autovacuum settings** for append-only workload
2. **Update replica autovacuum settings** for read optimization  
3. **Test performance** at 250-300 req/s to verify timeout elimination
4. **Monitor Grafana** for elimination of periodic timeout spikes

### Expected Impact
- **Eliminate periodic timeout spikes** caused by autovacuum blocking writes
- **Improve write consistency** by reducing vacuum-induced blocking
- **Maintain query performance** through frequent ANALYZE of JSONB indexes
- **Enable sustained 300+ req/s** without autovacuum interference

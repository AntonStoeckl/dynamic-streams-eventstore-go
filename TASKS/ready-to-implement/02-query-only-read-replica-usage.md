## Query-Only Read Replica Usage for Pure Query Handlers
- **Created**: 2025-08-07
- **Priority**: Medium - Performance optimization for read-heavy workloads
- **Objective**: Implement dedicated read-only database connections for pure query handlers (non-command operations) to reduce load on primary database

### Current Architecture Analysis
- **Current Implementation**: Read/write splitting routes EventStore Query() operations to replica, but command handlers still use the same EventStore instance
- **Mixed Usage**: Command handlers use EventStore.Query() for business decisions (part of command processing) and EventStore.Append() for writes
- **Performance Opportunity**: Pure query handlers like `BooksInCirculation` could use dedicated read-only connections to further reduce primary load

### Future Implementation Approach
- **Query-Only Adapters**: Create specialized read-only database adapters that explicitly prevent write operations
- **Separate EventStore Instances**: Query handlers could use dedicated read-only EventStore instances backed by query-only adapters
- **Clear Architectural Separation**: Make read-only usage explicit rather than implicit routing within existing adapters
- **Replica Optimization**: Read replicas can be tuned specifically for query workloads, especially for handlers returning large datasets

### Files/Packages to Consider
1. **Query-Only Adapter Design**:
   - `eventstore/postgresengine/internal/adapters/readonly_pgx_adapter.go` - Read-only PGX adapter with Exec() disabled
   - `eventstore/postgresengine/internal/adapters/readonly_sql_adapter.go` - Read-only SQL adapter with Exec() disabled
   - `eventstore/postgresengine/internal/adapters/readonly_sqlx_adapter.go` - Read-only SQLX adapter with Exec() disabled

2. **Pure Query Handlers** (candidates for read-only EventStore):
   - `example/features/booksincirculation/` - Returns large datasets, perfect candidate for replica optimization
   - `example/features/bookslentout/` - Cross-entity queries that could benefit from replica tuning
   - `example/features/bookslentbyreader/` - Reader-specific queries that don't need write access

3. **EventStore Constructor Patterns**:
   - `eventstore/postgresengine/postgres.go` - Add `NewReadOnlyEventStoreFromPGXPool()` constructors
   - Query-only EventStore instances with compile-time prevention of Append() operations

### Benefits of Query-Only Approach
- **Explicit Read-Only Semantics**: Impossible to accidentally perform writes through query-only adapters
- **Primary Database Protection**: Pure query workloads completely offloaded from primary database
- **Replica Tuning**: Read replicas can be optimized specifically for large result set queries (different buffer settings, query cache, etc.)
- **Architectural Clarity**: Clear distinction between command processing (read/write) and pure query operations (read-only)
- **Performance Scaling**: Query handlers with large datasets (like `BooksInCirculation`) get dedicated resources

### Implementation Strategy
- **Fail-Fast Design**: Query-only adapters should panic or return errors if Exec() is called
- **Constructor Safety**: Read-only EventStore constructors prevent Append() method availability
- **Configuration Flexibility**: Allow query handlers to choose between shared EventStore (current) or dedicated read-only instances
- **Backward Compatibility**: Existing command handlers continue using current read/write splitting approach

### Success Criteria
- Query-only adapters explicitly prevent write operations at compile/runtime
- Pure query handlers can use dedicated read-only EventStore instances
- Primary database load reduced for query-intensive operations
- Clear architectural separation between command processing and pure queries
- Replica databases optimized specifically for query workloads

---

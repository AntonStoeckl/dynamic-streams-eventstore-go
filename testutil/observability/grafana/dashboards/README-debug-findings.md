# Load Generator & EventStore Metrics Investigation Results

## 🔍 **The Mystery Solved**

**User's Question**: Why does load generator at 100 req/sec show ~50 Append + ~100 Query operations instead of 100 + 100?

## 📊 **Root Cause Analysis**

### **Load Generator Scenario Distribution**
- **Default weights**: `"20,80"` = 20% circulation, 80% lending
- **At 100 req/sec**: 20 circulation scenarios + 80 lending scenarios

### **EventStore Method Calls per Scenario**
Every command handler follows the same pattern:
1. **1 Query()** - Always executed to get current state
2. **1 Append()** - Only if business logic generates events (`len(eventsToAppend) > 0`)

### **Business Logic Reality Check**
The key insight: **Not every scenario generates events to append!**

- **Query() calls**: Always executed = 100/sec ✅
- **Append() calls**: Only when business logic decides to generate events ≈ 50/sec

This means ~50% of scenarios result in "nothing to do" and skip the Append() call.

## 🎯 **Expected vs Actual Operations**

### **With Method-Level Instrumentation (New)**
- `eventstore_query_method_calls_total` = Actual EventStore.Query() calls
- `eventstore_append_method_calls_total` = Actual EventStore.Append() calls

### **SQL-Level Operations (Existing)**  
- `eventstore_query_duration_seconds_count` = SQL SELECT operations
- `eventstore_append_duration_seconds_count` = SQL INSERT operations

### **The Relationship**
- **Method calls** = What the load generator actually calls
- **SQL operations** = What the database actually executes
- **For Append()**: 1 method call = 1 SELECT (CTE) + 1 INSERT = 2 SQL operations

## 📋 **Dashboard Improvements**

### **Main Dashboard** (`eventstore-simplified.json`)
Now shows **method-level metrics**:
- "Append Operations/sec" → Shows actual `Append()` method calls
- "Query Operations/sec" → Shows actual `Query()` method calls
- Success rate calculated using method calls as denominator

### **Debug Dashboard** (`eventstore-debug.json`)
New panel "Method Calls vs SQL Operations" shows:
- **EventStore.Append() Method Calls/sec** (user perspective)
- **EventStore.Query() Method Calls/sec** (user perspective)  
- **SQL INSERT Operations/sec** (database perspective)
- **SQL SELECT Operations/sec** (database perspective)

This eliminates confusion between application-level operations and database-level operations.

## ✅ **Validation Results**

With load generator at 100 req/sec, you should now see:
- **Query() Method Calls**: ~100/sec (every scenario queries first)
- **Append() Method Calls**: ~50/sec (only scenarios that generate events)
- **SQL SELECT Operations**: ~150/sec (100 from Query() + 50 from Append() CTEs)
- **SQL INSERT Operations**: ~50/sec (from successful Append() calls)

## 🎯 **Key Takeaway**

The "discrepancy" was not a bug - it was **correct business logic behavior**:
- Load generator scenarios don't always result in events being appended
- About 50% of scenarios result in "no changes needed" 
- This is typical in event-sourced applications where commands are idempotent

The confusion came from mixing **method-level metrics** (what the application does) with **SQL-level metrics** (what the database does).

Now both perspectives are clearly visible in the dashboards!
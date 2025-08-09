## Re-order Panels in Grafana Dashboard  
- **Created**: 2025-08-06
- **Priority**: Low - UX improvement for better dashboard usability
- **Objective**: Reorganize the 16-panel Grafana dashboard for better logical flow and visual hierarchy

### Current Dashboard Analysis
- **Current layout**: 16 panels in chronological order of implementation
- **User experience**: Related panels scattered across dashboard, non-intuitive flow
- **Visual hierarchy**: No clear grouping of related metrics (operations, errors, performance, context)

### Files/Packages to Review in Next Session
1. **Dashboard Configuration**:
   - `testutil/observability/grafana/dashboards/eventstore-dashboard.json` (panel positioning)
   - Panel `gridPos` coordinates and sizing for logical grouping

2. **Current Panel Inventory** (for reorganization planning):
   - **Operations**: Successful Append/Query Ops/sec (panels 1,2)  
   - **Errors**: Append/Query Errors/sec (panels 3,4)
   - **Performance**: Average durations (panels 7,8), SQL ops/sec (panels 5,6)
   - **Business Logic**: Command operations (panels 9,10), concurrency conflicts (panel 11), idempotent (panel 12)
   - **Context Errors**: Canceled operations (panels 13,14), timeout operations (panels 15,16)

### Next Session Implementation Plan
1. **Design Logical Groupings**: Group related panels into visual sections
2. **Proposed Layout**:
   - **Top Row**: Primary operations (successful append/query ops/sec)
   - **Second Row**: Performance metrics (average durations, P95/P99 if added)
   - **Third Row**: Error handling (errors/sec, concurrency conflicts)
   - **Fourth Row**: Context management (canceled, timeout operations)
   - **Bottom Row**: Business logic (command operations, idempotent)
3. **Update Panel Coordinates**: Modify `gridPos` for each panel
4. **Consider Panel Sizing**: Optimize panel widths for related metrics
5. **Visual Consistency**: Ensure consistent color schemes within groups

### Proposed Grouping Strategy
- **Core Operations Group**: Focus on primary EventStore operations and performance
- **Error Handling Group**: All error scenarios including business rules and infrastructure
- **Context Management Group**: Cancellation and timeout patterns
- **Business Logic Group**: Domain-specific operations and idempotency

### Success Criteria
- Logical flow from most important to least important metrics
- Related panels grouped visually 
- Improved dashboard usability for operations monitoring
- Maintained functionality of all 16 panels
- Clean visual layout with consistent spacing

---

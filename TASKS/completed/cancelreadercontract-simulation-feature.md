## Add CancelReaderContract to Load Generator Simulation
- **Completed**: 2025-08-09 (review confirmed)
- **Description**: Implemented CancelReaderContract scenario for realistic reader churn and lifecycle management
- **Problem Solved**: Missing reader cancellation functionality causing unrealistic reader accumulation without natural churn

### Implementation Completed
1. **ScenarioCancelReader**: Added to scenario types and all switch statements
2. **executeCancelReaderContract**: Implemented method using cancelReaderHandler
3. **State Tracking**: Added RemoveReader call in updateSimulationState 
4. **Handler Integration**: Added import, struct field, constructor, and observability options
5. **Weight Balance**: Fixed RegisterReader vs CancelReader balance for realistic growth

### Features Added
- **Natural Reader Churn**: Readers can now be canceled continuously with proper weighting
- **Threshold-Based Logic**: 
  - Below MinReaders: RegisterReader=5, CancelReader=0 (fast catch-up)
  - Normal range: RegisterReader=3, CancelReader=1 (3:1 ratio for steady growth)
  - At MaxReaders: RegisterReader=0, CancelReader=3 (force population down)
- **Business Logic Handling**: Existing CancelReaderContract feature handles cases with active lendings
- **State Consistency**: Canceled readers removed from state with lending cleanup

### Files Modified
- `example/simulation/cmd/scenarios.go` - Added ScenarioCancelReader type, weights, and scenario creation
- `example/simulation/cmd/simulation.go` - Added handler, execute method, state updates, and observability
- `example/simulation/cmd/state.go` - RemoveReader method already existed and works correctly

### Result
- Realistic reader lifecycle with both registrations and cancellations
- Steady reader growth toward MaxReaders with minimal natural churn
- Proper population self-regulation when hitting limits

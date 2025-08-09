## Enhance CancelReaderContract Business Logic Validation
- **Created**: 2025-08-09  
- **Priority**: Medium - Business rule completeness and domain integrity
- **Objective**: Implement proper business rule validation in CancelReaderContract command handler to ensure readers have zero books currently borrowed before allowing contract cancellation

### Current Problem Analysis
- **Missing Business Rule**: CancelReaderContract command handler may not properly validate that a reader has returned all borrowed books
- **Domain Integrity**: Contract cancellation should be prevented if reader still has outstanding book loans
- **Event Processing Required**: Must process lending/returning events to determine current borrowed book count
- **Business Logic Gap**: Command handler needs to project current reader state from event history

### Implementation Requirements

#### Core Business Rule
- **Zero Outstanding Loans**: Reader must have zero books currently borrowed before contract cancellation
- **Event-Based Validation**: Process all lending and return events for the specific reader to calculate current loan count
- **Proper Error Handling**: Return appropriate business rule violation when reader has outstanding loans
- **Idempotency Support**: Handle cases where reader is already canceled (no-op scenario)

#### Files to Modify
1. **Command Handler Logic**:
   - `example/features/cancelreadercontract/decide.go` - Add borrowed books count validation
   - `example/features/cancelreadercontract/command_handler.go` - Ensure proper error handling

2. **Event Processing Enhancement**:
   - Review lending/return event processing logic in the decide function
   - Implement counter for outstanding loans based on event history
   - Handle edge cases (books lent but never returned, multiple lending cycles)

3. **Test Coverage**:
   - `example/features/cancelreadercontract/` - Add test cases for business rule validation
   - Test scenarios: reader with outstanding loans, reader with zero loans, already canceled reader

### Implementation Approach

#### Event Processing Logic
```go
// Pseudo-code for enhanced business logic validation
func (d CancelReaderContractDecider) Decide(events []core.DomainEvent, command CancelReaderContractCommand) DecisionResult {
    outstandingLoans := 0
    readerCanceled := false
    
    // Process all reader-related events
    for _, event := range events {
        switch event.EventType() {
        case core.BookCopyLentToReaderEventType:
            if lendingEvent.ReaderID == command.ReaderID {
                outstandingLoans++
            }
        case core.BookCopyReturnedByReaderEventType:
            if returnEvent.ReaderID == command.ReaderID {
                outstandingLoans--
            }
        case core.ReaderContractCanceledEventType:
            if cancelEvent.ReaderID == command.ReaderID {
                readerCanceled = true
            }
        }
    }
    
    // Business rule validation
    if readerCanceled {
        return DecisionResult{Success: true} // Idempotent - already canceled
    }
    
    if outstandingLoans > 0 {
        return DecisionResult{
            Success: false,
            Error: fmt.Errorf("cannot cancel reader contract: reader has %d outstanding book loans", outstandingLoans),
        }
    }
    
    // Proceed with cancellation
    return DecisionResult{Success: true, Events: []core.DomainEvent{...}}
}
```

#### Filter Enhancement
- **Comprehensive Event Filtering**: Ensure query filter captures all relevant events for the reader
- **Cross-Entity Events**: Include both reader-specific and lending-related events
- **Proper Predicates**: Use correct `ReaderID` predicates for all relevant event types

### Business Rule Validation
- **Outstanding Loan Prevention**: Reader cannot be canceled while having borrowed books
- **Data Consistency**: Event processing must accurately reflect current loan state
- **Error Messages**: Clear, descriptive error messages for business rule violations
- **Edge Case Handling**: Handle scenarios like incomplete return processes or data inconsistencies

### Success Criteria
- **Zero Outstanding Loans Validation**: Command handler properly prevents cancellation when reader has borrowed books
- **Accurate Event Processing**: Lending and return events correctly calculated to determine current loan count
- **Proper Error Handling**: Clear business rule violation errors when cancellation is invalid
- **Idempotency Support**: Already-canceled readers handled gracefully
- **Comprehensive Test Coverage**: Test cases covering all business rule scenarios
- **Domain Integrity**: Reader contract cancellation maintains proper business domain constraints

---

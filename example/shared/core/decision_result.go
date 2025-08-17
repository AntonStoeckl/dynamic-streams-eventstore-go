package core

// DecisionResult represents the outcome of a business decision in a Decide function.
// This enables type-safe, functional programming style decision modeling.
//
// IMPORTANT: DecisionResult should only be constructed using the provided factory methods:
// IdempotentDecision(), SuccessDecision(event), or ErrorDecision(event).
// Do not construct DecisionResult directly to ensure type safety.
type DecisionResult struct {
	Outcome string      // "idempotent", "success", or "error"
	Event   DomainEvent // nil for idempotent decisions
	Err     error
}

const (
	idempotentOutcome = "idempotent"
	successOutcome    = "success"
	errorOutcome      = "error"
)

// IdempotentDecision creates a DecisionResult indicating no state change is needed.
func IdempotentDecision() DecisionResult {
	return DecisionResult{
		Outcome: idempotentOutcome,
		Event:   nil,
	}
}

// SuccessDecision creates a DecisionResult indicating a successful state change with an event to append.
func SuccessDecision(event DomainEvent) DecisionResult {
	return DecisionResult{
		Outcome: successOutcome,
		Event:   event,
	}
}

// ErrorDecision creates a DecisionResult indicating a business rule violation with an error event to append.
func ErrorDecision(event DomainEvent, err error) DecisionResult {
	return DecisionResult{
		Outcome: errorOutcome,
		Event:   event,
		Err:     err,
	}
}

// HasEventToAppend returns true if there is an event to append to the event store.
func (r DecisionResult) HasEventToAppend() bool {
	return r.Outcome != idempotentOutcome
}

// HasError returns the error if there is one, otherwise nil.
func (r DecisionResult) HasError() error {
	if r.Outcome == errorOutcome {
		return r.Err
	}

	return nil
}

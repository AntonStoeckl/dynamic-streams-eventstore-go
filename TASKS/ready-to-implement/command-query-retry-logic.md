## Implement Retry Logic for Command/Query Handlers
- **Created**: 2025-08-06
- **Priority**: Medium - Production resilience improvement
- **Objective**: Add configurable retry logic with exponential backoff to command and query handlers for handling transient database errors and concurrency conflicts

### Current Problem Analysis
- **Current behavior**: Single-attempt operations fail immediately on transient errors (network issues, temporary DB unavailability, high concurrency)
- **Production impact**: Reduces system resilience under load or during infrastructure issues
- **User experience**: Operations that could succeed with retry fail permanently
- **Missing patterns**: No standardized retry configuration across different handler types

### Files/Packages to Review in Next Session
1. **Handler Infrastructure**:
   - `example/shared/shell/command_handler_observability.go` (add retry status tracking)
   - `example/features/*/command_handler.go` (6 command handlers to update)
   - `example/features/*/query_handler.go` (3 query handlers to update)

2. **Error Classification** (determine retryable vs non-retryable):
   - `eventstore/postgresengine/postgres.go` (context errors, DB errors, concurrency conflicts)
   - `eventstore/errors.go` (ErrConcurrencyConflict, ErrInvalidPayloadJSON patterns)

3. **Configuration**:
   - `example/shared/shell/config/` (add retry configuration options)
   - Consider environment variables: `MAX_RETRIES`, `RETRY_BASE_DELAY`, `RETRY_MAX_DELAY`

### Next Session Implementation Plan
1. **Design Retry Configuration**: Create RetryConfig struct with max attempts, base delay, max delay, backoff multiplier
2. **Error Classification**: Implement `IsRetryableError()` helper to distinguish permanent vs transient failures
3. **Retry Logic Pattern**: Create reusable retry wrapper that works with existing handler patterns
4. **Observability Integration**: Add retry attempt counts, failure reasons, backoff delays to metrics
5. **Handler Updates**: Integrate retry logic into all command and query handlers
6. **Configuration Management**: Add retry settings to shell/config with sensible defaults

### Implementation Approach
- **Retryable Errors**: Context timeouts, temporary DB connection issues, some concurrency conflicts
- **Non-Retryable Errors**: Business rule violations, invalid payloads, permanent authentication issues
- **Backoff Strategy**: Exponential with jitter to avoid thundering herd effects
- **Observability**: Track retry attempts, success after retry, permanent failures
- **Configuration**: Environment-driven with production-safe defaults

### Success Criteria
- Configurable retry logic across all handlers
- Improved resilience under temporary infrastructure issues
- Proper error classification (retryable vs permanent)
- Complete observability for retry patterns
- Zero breaking changes to existing handler APIs


---

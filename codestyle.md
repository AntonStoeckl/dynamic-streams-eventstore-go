# Code Style Guide

This document outlines code style conventions for this project beyond what `gofmt` provides.

## Code Formatting
- Follow `gofmt`
- Follow `goimports` for imports structuring

### Additional Formating Preferences

#### Return Statements
- Always put an empty line before return statements, unless it's the only line in a block (if, function, ...)

#### Switch/Case blocks
- Always separate cases with an empty line in between

#### Functions
- If a function has more than two input parameters, wrap the input parameters
- If a function has more than two return parameters, wrap the return parameters
- If a function is more than 130 characters long, also wrap parameters, prefer wrapping the input parameters
- In any of those cases, start the function body with an empty line
- Ignore this for *_test.go!

Full example:

```go
func SomeFunction(
    a string,
    b string,
    c string,
) (
    string,
    string,
    string,
) {

    r1, r2, r3 = c, b, a

    return r1, r2, r3
}
```

#### Function Calls
- If any function call has more than three parameters, wrap the parameters, starting with the first parameter
- Ignore this for *_test.go files unless the line is longer than 130 characters

#### Grouping of Err, Const, Var
- Should be all on top (after package)
- If one file has more than one of each, merge them in ()
- Err comes first, const second, Var third
- Next come types, then methods and functions
- If a file has multiple types with receiver methods, the order is: type, its methods, type, its methods

#### Type aliases
- Exported functions and methods should not return primitives, except bool
- Instead, an exported type alias should be created which is descriptive followed by its type (e.g., MaxSequenceNumUint)
- This can be ignored if the function or method only returns one parameter (except error) and the function name is descriptive enough
- The same for unexported methods, an unexported type alias should be used here
- Apply common sense, sometimes this might not make sense

## Naming Conventions

### Variables and Functions
- Use descriptive names that clearly indicate purpose
- Prefer full words over abbreviations (e.g., `eventStore` not `es`)
- Use camelCase for unexported identifiers, PascalCase for exported ones

### Types and Interfaces
- Interface names should describe behavior, often ending in `-er` (e.g., `DBAdapter`, `Wrapper`)
- Struct names should be nouns describing the entity
- Avoid generic names like `Manager`, `Handler` unless they accurately describe the role

## Error Handling Patterns

### Sentinel Errors
- Define sentinel errors as package-level variables with `Err` prefix
- Use descriptive error messages that help with debugging
- Example: `ErrConcurrencyConflict`, `ErrEmptyTableNameSupplied`

### Error Assertions in Tests
- Use single `assert.ErrorContains()` for expected errors
- Always use `err.Error()` when asserting sentinel error messages
- Trust developers to check errors properly—avoid over-assertion

## Interface Design Principles

### Adapter Pattern
- Keep interfaces focused and minimal (e.g., `DBAdapter` with just `Query()` and `Exec()`)
- Use composition over inheritance
- Ensure all implementations provide equivalent functionality

### Abstraction Layers
- Create clear boundaries between domain logic and infrastructure
- Use wrapper interfaces for testing (e.g., `Wrapper` interface for database adapters)

## Test Organization and Naming

### Test Function Naming
- Always use `Test_*` pattern with descriptive names
- For generic tests: `Test_Generic_*` prefix
- Use underscores to separate logical parts: `Test_BuildStorableEvent_InvalidJSON`

### Test Categories
- **Generic tests**: Adapter-independent factory tests (prefixed `Test_Generic_*`)
- **Functional tests**: Adapter-dependent integration tests
- Separate test files for different concerns (`postgres_test.go`, `postgres_generic_test.go`)

### Test Assertions
- Don't test for empty structs or zero values when errors occur
- Focus on the specific behavior being tested
- Use environment variables for adapter switching in functional tests

## Comment and Documentation Standards

### Function Comments
- Document exported functions with clear, concise descriptions
- Focus on what the function does and when to use it
- Include parameter and return value descriptions for complex functions

### Package Comments
- Each package should have a clear purpose statement
- Document the main abstractions and patterns used
- Explain integration points with other packages

## Package Organization Preferences

### Directory Structure
- Separate domain logic from infrastructure concerns
- Use internal packages for implementation details
- Group related functionality in logical packages

## Functional Options Patterns

### Factory Functions
- Use functional options for optional configuration
- Provide sensible defaults
- Example: `WithTableName(tableName)`, `WithLogger(logger)`

### Option Function Design
- Return functions that modify the target struct
- Use clear, descriptive names starting with `With`
- Handle validation within option functions when appropriate

## Database and SQL Patterns

### Query Building
- Use builder patterns for dynamic queries (e.g., `goqu`)
- Prefer `fmt.Sprintf` for JSON predicate building over prepared statements
- Keep SQL queries readable and well-formatted

### Adapter Implementation
- Ensure all database adapters provide equivalent functionality
- Use consistent error handling across adapters
- Implement proper resource cleanup (defer statements)
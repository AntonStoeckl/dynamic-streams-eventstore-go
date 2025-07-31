package config

// PostgresTestDSN returns the DSN for the test database
func PostgresTestDSN() string {
	return "postgres://test:test@localhost:5432/eventstore?sslmode=disable"
}

// PostgresBenchmarkDSN returns the DSN for the benchmark database
func PostgresBenchmarkDSN() string {
	return "postgres://test:test@localhost:5433/eventstore?sslmode=disable"
}

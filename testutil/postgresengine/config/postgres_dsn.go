package config

// PostgresSingleDSN returns the DSN for the test database.
func PostgresSingleDSN() string {
	return "postgres://test:test@localhost:5432/eventstore?sslmode=disable"
}

// PostgresPrimaryDSN returns the DSN for the primary benchmark database.
func PostgresPrimaryDSN() string {
	return "postgres://test:test@localhost:5433/eventstore?sslmode=disable"
}

// PostgresReplicaDSN returns the DSN for the replica benchmark database.
func PostgresReplicaDSN() string {
	return "postgres://test:test@localhost:5434/eventstore?sslmode=disable"
}

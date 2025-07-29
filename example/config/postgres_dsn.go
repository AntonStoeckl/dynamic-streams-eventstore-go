package config

func PostgresTestDSN() string {
	return "postgres://test:test@localhost:5432/eventstore?sslmode=disable"
}

func PostgresBenchmarkDSN() string {
	return "postgres://test:test@localhost:5433/eventstore?sslmode=disable"
}

package config

import (
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresPGXPoolTestConfig creates a pgxpool.Config for the test database.
func PostgresPGXPoolTestConfig() *pgxpool.Config {
	const defaultMaxConnections = int32(8)
	const defaultMinConnections = int32(2)
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5
	const defaultHealthCheckPeriod = time.Minute
	const defaultConnectTimeout = time.Second * 5

	dbConfig, err := pgxpool.ParseConfig(PostgresTestDSN())
	if err != nil {
		log.Fatal("Failed to create a config, error: ", err)
	}

	dbConfig.MaxConns = defaultMaxConnections
	dbConfig.MinConns = defaultMinConnections
	dbConfig.MaxConnLifetime = defaultMaxConnLifetime
	dbConfig.MaxConnIdleTime = defaultMaxConnIdleTime
	dbConfig.HealthCheckPeriod = defaultHealthCheckPeriod
	dbConfig.ConnConfig.ConnectTimeout = defaultConnectTimeout

	return dbConfig
}

// PostgresPGXPoolBenchmarkConfig creates a pgxpool.Config for the benchmark database.
func PostgresPGXPoolBenchmarkConfig() *pgxpool.Config {
	const defaultMaxConnections = int32(8)
	const defaultMinConnections = int32(2)
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5
	const defaultHealthCheckPeriod = time.Minute
	const defaultConnectTimeout = time.Second * 5

	dbConfig, err := pgxpool.ParseConfig(PostgresBenchmarkDSN())
	if err != nil {
		log.Fatal("Failed to create a config, error: ", err)
	}

	dbConfig.MaxConns = defaultMaxConnections
	dbConfig.MinConns = defaultMinConnections
	dbConfig.MaxConnLifetime = defaultMaxConnLifetime
	dbConfig.MaxConnIdleTime = defaultMaxConnIdleTime
	dbConfig.HealthCheckPeriod = defaultHealthCheckPeriod
	dbConfig.ConnConfig.ConnectTimeout = defaultConnectTimeout

	return dbConfig
}

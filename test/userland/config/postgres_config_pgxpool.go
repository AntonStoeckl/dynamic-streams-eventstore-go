package config

import (
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func PostgresPGXPoolTestConfig() *pgxpool.Config {
	const defaultMaxConnections = int32(8)
	const defaultMinConnections = int32(2)
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5
	const defaultHealthCheckPeriod = time.Minute
	const defaultConnectTimeout = time.Second * 5

	// Your own Database URL
	const DatabaseUrl string = "postgres://test:test@localhost:5432/eventstore?"

	dbConfig, err := pgxpool.ParseConfig(DatabaseUrl)
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

func PostgresPGXPoolBenchmarkConfig() *pgxpool.Config {
	const defaultMaxConnections = int32(8)
	const defaultMinConnections = int32(2)
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5
	const defaultHealthCheckPeriod = time.Minute
	const defaultConnectTimeout = time.Second * 5

	// Your own Database URL
	const DatabaseUrl string = "postgres://test:test@localhost:5433/eventstore?"

	dbConfig, err := pgxpool.ParseConfig(DatabaseUrl)
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

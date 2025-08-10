package config

import (
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresPGXPoolSingleConfig creates a pgxpool.Config for a single database.
func PostgresPGXPoolSingleConfig() *pgxpool.Config {
	const defaultMaxConnections = int32(50)
	const defaultMinConnections = int32(10)
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5
	const defaultHealthCheckPeriod = time.Minute
	const defaultConnectTimeout = time.Second * 5

	dbConfig, err := pgxpool.ParseConfig(PostgresSingleDSN())
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

// PostgresPGXPoolPrimaryConfig creates a pgxpool.Config for the primary node of a replicated database.
func PostgresPGXPoolPrimaryConfig() *pgxpool.Config {
	const defaultMaxConnections = int32(60)
	const defaultMinConnections = int32(20)
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5
	const defaultHealthCheckPeriod = time.Minute
	const defaultConnectTimeout = time.Second * 5

	dbConfig, err := pgxpool.ParseConfig(PostgresPrimaryDSN())
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

// PostgresPGXPoolReplicaConfig creates a pgxpool.Config for the replica node of a replicated database.
func PostgresPGXPoolReplicaConfig() *pgxpool.Config {
	const defaultMaxConnections = int32(60)
	const defaultMinConnections = int32(20)
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5
	const defaultHealthCheckPeriod = time.Minute
	const defaultConnectTimeout = time.Second * 5

	dbConfig, err := pgxpool.ParseConfig(PostgresReplicaDSN())
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

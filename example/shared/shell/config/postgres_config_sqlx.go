package config

import (
	"context"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // postgres driver
)

// PostgresSQLXSingleConfig creates a configured *sqlx.DB for a single database.
func PostgresSQLXSingleConfig() *sqlx.DB {
	const defaultMaxOpenConnections = 50
	const defaultMaxIdleConnections = 2 // OPTIMIZED: Reduced from 10 to 2 (consistent with pgx.Pool optimization)
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5

	db, err := sqlx.Open("postgres", PostgresSingleDSN())
	if err != nil {
		log.Fatal("Failed to open database connection, error: ", err)
	}

	// Configure connection pool settings
	db.SetMaxOpenConns(defaultMaxOpenConnections)
	db.SetMaxIdleConns(defaultMaxIdleConnections)
	db.SetConnMaxLifetime(defaultMaxConnLifetime)
	db.SetConnMaxIdleTime(defaultMaxConnIdleTime)

	// Test the connection
	if pingErr := db.PingContext(context.Background()); pingErr != nil {
		log.Fatal("Failed to ping database, error: ", pingErr)
	}

	return db
}

// PostgresSQLXPrimaryConfig creates a configured *sqlx.DB for the primary node of a replicated database.
func PostgresSQLXPrimaryConfig() *sqlx.DB {
	const defaultMaxOpenConnections = 60
	const defaultMaxIdleConnections = 2 // OPTIMIZED: Reduced from 20 to 2 (equivalent to pgx.Pool MinConnections)
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5

	db, err := sqlx.Open("postgres", PostgresPrimaryDSN())
	if err != nil {
		log.Fatal("Failed to open database connection, error: ", err)
	}

	// Configure connection pool settings
	db.SetMaxOpenConns(defaultMaxOpenConnections)
	db.SetMaxIdleConns(defaultMaxIdleConnections)
	db.SetConnMaxLifetime(defaultMaxConnLifetime)
	db.SetConnMaxIdleTime(defaultMaxConnIdleTime)

	// Test the connection
	if pingErr := db.PingContext(context.Background()); pingErr != nil {
		log.Fatal("Failed to ping database, error: ", pingErr)
	}

	return db
}

// PostgresSQLXReplicaConfig creates a configured *sqlx.DB for the replica node of a replicated database.
func PostgresSQLXReplicaConfig() *sqlx.DB {
	const defaultMaxOpenConnections = 60
	const defaultMaxIdleConnections = 2 // OPTIMIZED: Reduced from 20 to 2 (equivalent to pgx.Pool MinConnections)
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5

	db, err := sqlx.Open("postgres", PostgresReplicaDSN())
	if err != nil {
		log.Fatal("Failed to open database connection, error: ", err)
	}

	// Configure connection pool settings
	db.SetMaxOpenConns(defaultMaxOpenConnections)
	db.SetMaxIdleConns(defaultMaxIdleConnections)
	db.SetConnMaxLifetime(defaultMaxConnLifetime)
	db.SetConnMaxIdleTime(defaultMaxConnIdleTime)

	// Test the connection
	if pingErr := db.PingContext(context.Background()); pingErr != nil {
		log.Fatal("Failed to ping database, error: ", pingErr)
	}

	return db
}

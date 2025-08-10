package config

import (
	"context"
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq" // postgres driver
)

// PostgresSQLDBSingleConfig creates a configured *sql.DB for a single database.
func PostgresSQLDBSingleConfig() *sql.DB {
	const defaultMaxOpenConnections = 50
	const defaultMaxIdleConnections = 2 // OPTIMIZED: Reduced from 10 to 2 (consistent with pgx.Pool optimization)
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5

	db, err := sql.Open("postgres", PostgresSingleDSN())
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

// PostgresSQLDBPrimaryConfig creates a configured *sql.DB for the primary node of a replicated database.
func PostgresSQLDBPrimaryConfig() *sql.DB {
	const defaultMaxOpenConnections = 60
	const defaultMaxIdleConnections = 2 // OPTIMIZED: Reduced from 20 to 2 (equivalent to pgx.Pool MinConnections)
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5

	db, err := sql.Open("postgres", PostgresPrimaryDSN())
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

// PostgresSQLDBReplicaConfig creates a configured *sql.DB for the replica node of a replicated database.
func PostgresSQLDBReplicaConfig() *sql.DB {
	const defaultMaxOpenConnections = 60
	const defaultMaxIdleConnections = 2 // OPTIMIZED: Reduced from 20 to 2 (equivalent to pgx.Pool MinConnections)
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5

	db, err := sql.Open("postgres", PostgresReplicaDSN())
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

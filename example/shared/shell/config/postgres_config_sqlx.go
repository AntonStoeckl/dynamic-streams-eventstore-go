package config

import (
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // postgres driver
)

// PostgresSQLXTestConfig creates a configured *sqlx.DB for the test database.
func PostgresSQLXTestConfig() *sqlx.DB {
	const defaultMaxOpenConnections = 8
	const defaultMaxIdleConnections = 2
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5

	db, err := sqlx.Open("postgres", PostgresTestDSN())
	if err != nil {
		log.Fatal("Failed to open database connection, error: ", err)
	}

	// Configure connection pool settings
	db.SetMaxOpenConns(defaultMaxOpenConnections)
	db.SetMaxIdleConns(defaultMaxIdleConnections)
	db.SetConnMaxLifetime(defaultMaxConnLifetime)
	db.SetConnMaxIdleTime(defaultMaxConnIdleTime)

	// Test the connection
	if pingErr := db.Ping(); pingErr != nil {
		log.Fatal("Failed to ping database, error: ", pingErr)
	}

	return db
}

// PostgresSQLXBenchmarkConfig creates a configured *sqlx.DB for the benchmark database.
func PostgresSQLXBenchmarkConfig() *sqlx.DB {
	const defaultMaxOpenConnections = 8
	const defaultMaxIdleConnections = 2
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5

	db, err := sqlx.Open("postgres", PostgresBenchmarkDSN())
	if err != nil {
		log.Fatal("Failed to open database connection, error: ", err)
	}

	// Configure connection pool settings
	db.SetMaxOpenConns(defaultMaxOpenConnections)
	db.SetMaxIdleConns(defaultMaxIdleConnections)
	db.SetConnMaxLifetime(defaultMaxConnLifetime)
	db.SetConnMaxIdleTime(defaultMaxConnIdleTime)

	// Test the connection
	if pingErr := db.Ping(); pingErr != nil {
		log.Fatal("Failed to ping database, error: ", pingErr)
	}

	return db
}

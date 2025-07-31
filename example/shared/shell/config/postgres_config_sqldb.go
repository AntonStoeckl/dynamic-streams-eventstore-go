package config

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq" // postgres driver
)

// PostgresSQLDBTestConfig creates a configured *sql.DB for the test database
func PostgresSQLDBTestConfig() *sql.DB {
	const defaultMaxOpenConnections = 8
	const defaultMaxIdleConnections = 2
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5

	db, err := sql.Open("postgres", PostgresTestDSN())
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

// PostgresSQLDBBenchmarkConfig creates a configured *sql.DB for the benchmark database
func PostgresSQLDBBenchmarkConfig() *sql.DB {
	const defaultMaxOpenConnections = 8
	const defaultMaxIdleConnections = 2
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5

	db, err := sql.Open("postgres", PostgresBenchmarkDSN())
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

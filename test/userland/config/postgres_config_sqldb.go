package config

import (
	"database/sql"
	"log"
	"time"
)

func PostgresSQLDBTestConfig() *sql.DB {
	const defaultMaxOpenConnections = 8
	const defaultMaxIdleConnections = 2
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5

	// Your own Database URL
	const DatabaseUrl string = "postgres://test:test@localhost:5432/eventstore?sslmode=disable"

	db, err := sql.Open("postgres", DatabaseUrl)
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

func PostgresSQLDBBenchmarkConfig() *sql.DB {
	const defaultMaxOpenConnections = 8
	const defaultMaxIdleConnections = 2
	const defaultMaxConnLifetime = time.Hour
	const defaultMaxConnIdleTime = time.Minute * 5

	// Your own Database URL
	const DatabaseUrl string = "postgres://test:test@localhost:5433/eventstore?sslmode=disable"

	db, err := sql.Open("postgres", DatabaseUrl)
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

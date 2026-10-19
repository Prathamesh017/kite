package models

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	_ "github.com/mattn/go-sqlite3"
)

// InitDB opens or creates the memory database.
// It creates the .kite directory if it doesn't exist.
// Schema is created if the database is new.
func InitDB() (*sql.DB, error) {
	// Get the working directory (where kite was started)
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Create .kite directory if it doesn't exist
	kiteDir := filepath.Join(wd, ".kite")
	if err := os.MkdirAll(kiteDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create .kite directory: %w", err)
	}

	// Open or create the database
	dbPath := filepath.Join(kiteDir, "memory.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create tables if they don't exist
	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return db, nil
}

// createTables creates the schema if it doesn't exist.
func createTables(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS file_memories (
		file_path TEXT PRIMARY KEY,
		file_name TEXT NOT NULL,
		file_hash TEXT NOT NULL,
		summary TEXT NOT NULL,
		learned_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		access_count INTEGER DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_file_memories_file_name ON file_memories(file_name);
	CREATE INDEX IF NOT EXISTS idx_file_memories_updated_at ON file_memories(updated_at DESC);
	CREATE INDEX IF NOT EXISTS idx_file_memories_access_count ON file_memories(access_count DESC);
	`

	_, err := db.Exec(schema)
	return err
}

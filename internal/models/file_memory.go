package models

import (
	"database/sql"
	"time"
)

// FileMemory represents what we've learned about a file.
// When the agent reads a file, we summarize it and store the summary here.
// Next time the agent (or a tool) references the file, we can check if our
// knowledge is still fresh (hash unchanged) or stale (file changed).
type FileMemory struct {
	FileName     string    // Just the filename (e.g., "main.go")
	FilePath     string    // Absolute path to the file
	FileHash     string    // Hash of file content when we read it (detect changes)
	Summary      string    // What we learned from reading this file
	LearnedAt    time.Time // When we first learned this
	UpdatedAt    time.Time // Last time we accessed or updated it
	AccessCount  int       // How many times we've used this memory
}

// CreateFileMemory inserts a new file memory into the database.
// Used when we read a file for the first time and summarize it.
func CreateFileMemory(db *sql.DB, fileName, filePath, fileHash, summary string) error {
	now := time.Now()
	query := `
		INSERT INTO file_memories (file_name, file_path, file_hash, summary, learned_at, updated_at, access_count)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	_, err := db.Exec(query, fileName, filePath, fileHash, summary, now, now, 0)
	return err
}

// GetFileMemory retrieves a file memory by path.
// Returns nil if not found.
func GetFileMemory(db *sql.DB, filePath string) (*FileMemory, error) {
	query := `
		SELECT file_name, file_path, file_hash, summary, learned_at, updated_at, access_count
		FROM file_memories
		WHERE file_path = ?
	`
	row := db.QueryRow(query, filePath)

	var fm FileMemory
	err := row.Scan(&fm.FileName, &fm.FilePath, &fm.FileHash, &fm.Summary, &fm.LearnedAt, &fm.UpdatedAt, &fm.AccessCount)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found, no error
		}
		return nil, err
	}
	return &fm, nil
}

// UpdateFileMemory updates an existing file memory.
// Used when we re-read a file and the hash changed, or we want to update the summary.
func UpdateFileMemory(db *sql.DB, filePath, fileHash, summary string) error {
	now := time.Now()
	query := `
		UPDATE file_memories
		SET file_hash = ?, summary = ?, updated_at = ?, access_count = access_count + 1
		WHERE file_path = ?
	`
	_, err := db.Exec(query, fileHash, summary, now, filePath)
	return err
}

// TouchFileMemory increments the access count when we use a memory.
// Used when the agent references a file we've already summarized.
func TouchFileMemory(db *sql.DB, filePath string) error {
	now := time.Now()
	query := `
		UPDATE file_memories
		SET updated_at = ?, access_count = access_count + 1
		WHERE file_path = ?
	`
	_, err := db.Exec(query, now, filePath)
	return err
}

// DeleteFileMemory removes a file memory from the database.
func DeleteFileMemory(db *sql.DB, filePath string) error {
	query := `DELETE FROM file_memories WHERE file_path = ?`
	_, err := db.Exec(query, filePath)
	return err
}

// ListAllFileMemories returns all file memories in the database.
func ListAllFileMemories(db *sql.DB) ([]*FileMemory, error) {
	query := `
		SELECT file_name, file_path, file_hash, summary, learned_at, updated_at, access_count
		FROM file_memories
		ORDER BY updated_at DESC
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var memories []*FileMemory
	for rows.Next() {
		var fm FileMemory
		err := rows.Scan(&fm.FileName, &fm.FilePath, &fm.FileHash, &fm.Summary, &fm.LearnedAt, &fm.UpdatedAt, &fm.AccessCount)
		if err != nil {
			return nil, err
		}
		memories = append(memories, &fm)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return memories, nil
}

package sqlite

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"github.com/pancakeswya/gopad/pkg/livecode"
	"os"
	"path"
)

const (
	dbName       = "db.sql"
	defaultDbDir = "db_data"
)

type Database struct {
	db *sql.DB
}

func NewDatabase() (*Database, error) {
	dbPath, err := getOrCreateDefaultDbPath()
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if err = runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}
	return &Database{db: db}, nil
}

func (database *Database) Close() {
	database.db.Close()
}

func (database *Database) Load(id string) (*livecode.Document, error) {
	var doc livecode.Document
	err := database.db.
		QueryRow(`
			SELECT text, language 
			FROM document
			WHERE id = ?`, id).
		Scan(&doc.Text, &doc.Language)
	if err != nil {
		return nil, fmt.Errorf("failed to load document: %w", err)
	}
	return &doc, nil
}

func (database *Database) Store(id string, document *livecode.Document) error {
	result, err := database.db.Exec(`
		INSERT INTO document (id, text, language)
		VALUES (?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			text = excluded.text,
			language = excluded.language`,
		id,
		document.Text,
		document.Language,
	)
	if err != nil {
		return fmt.Errorf("failed to store document: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("expected store() to affect 1 row, but it affected %database rows", rowsAffected)
	}
	return nil
}

func (database *Database) Count() (int, error) {
	var count int
	if err := database.db.QueryRow("SELECT count(*) FROM document").Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count documents: %w", err)
	}
	return count, nil
}

func runMigrations(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS document (
			id TEXT PRIMARY KEY,
			text TEXT NOT NULL,
			language TEXT
		)
	`)
	return err
}

func getOrCreateDefaultDbPath() (string, error) {
	dbPath := os.Getenv("DB_PATH")
	if len(dbPath) != 0 {
		return dbPath, nil
	}
	if err := os.MkdirAll(defaultDbDir, 0750); err != nil {
		return "", err
	}
	return path.Join(defaultDbDir, dbName), nil
}

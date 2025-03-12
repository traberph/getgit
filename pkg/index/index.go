package index

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
	"github.com/traberph/getgit/pkg/config"
	"github.com/traberph/getgit/pkg/sources"
)

// Manager handles the SQLite database operations for repository indexing
type Manager struct {
	db *sql.DB
}

// NewManager creates a new index manager instance
func NewManager() (*Manager, error) {
	dbPath, err := getDBPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get database path: %w", err)
	}

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	manager := &Manager{db: db}
	if err := manager.initDB(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	return manager, nil
}

// Close closes the database connection
func (m *Manager) Close() error {
	return m.db.Close()
}

// getDBPath returns the path to the index database
func getDBPath() (string, error) {
	cacheDir, err := config.GetCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cacheDir, "index.db"), nil
}

// initDB creates the necessary database tables if they don't exist
func (m *Manager) initDB() error {
	// Create tables if they don't exist
	schema := `
	CREATE TABLE IF NOT EXISTS repositories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		url TEXT NOT NULL,
		build TEXT,
		executable TEXT,
		source_file TEXT NOT NULL,
		source_name TEXT NOT NULL,
		load TEXT,
		UNIQUE(name, source_file)
	);
	CREATE INDEX IF NOT EXISTS idx_repo_name ON repositories(name);
	`

	_, err := m.db.Exec(schema)
	return err
}

// UpdateIndex updates the index database with the latest source information
func (m *Manager) UpdateIndex(sourceManager *sources.SourceManager) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Clear existing entries
	if _, err := tx.Exec("DELETE FROM repositories"); err != nil {
		return fmt.Errorf("failed to clear existing entries: %w", err)
	}

	// Insert new entries
	stmt, err := tx.Prepare(`
		INSERT INTO repositories (name, url, build, executable, source_file, source_name, load)
		VALUES (?, ?, NULLIF(TRIM(?), ''), NULLIF(TRIM(?), ''), ?, ?, NULLIF(TRIM(?), ''))
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	for _, source := range sourceManager.Sources {
		for _, repo := range source.Repos {
			_, err := stmt.Exec(
				repo.Name,
				repo.URL,
				repo.Build,
				repo.Executable,
				source.FilePath,
				source.Name,
				repo.Load,
			)
			if err != nil {
				return fmt.Errorf("failed to insert repository %s: %w", repo.Name, err)
			}
		}
	}

	return tx.Commit()
}

// FindRepository searches for a repository by name and returns all matching entries
func (m *Manager) FindRepository(name string) ([]RepoInfo, error) {
	rows, err := m.db.Query(`
		SELECT name, url, COALESCE(build, '') as build, COALESCE(executable, '') as executable, source_file, source_name, COALESCE(load, '') as load
		FROM repositories
		WHERE name COLLATE NOCASE = ?
	`, name)
	if err != nil {
		return nil, fmt.Errorf("failed to query repository %s: %w", name, err)
	}
	defer rows.Close()

	var repos []RepoInfo
	for rows.Next() {
		var repo RepoInfo
		err := rows.Scan(
			&repo.Name,
			&repo.URL,
			&repo.Build,
			&repo.Executable,
			&repo.SourceFile,
			&repo.SourceName,
			&repo.Load,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan repository row: %w", err)
		}
		repos = append(repos, repo)
	}

	return repos, rows.Err()
}

// RepoInfo represents repository information stored in the index
type RepoInfo struct {
	Name       string
	URL        string
	Build      string
	Executable string
	SourceFile string
	SourceName string
	Load       string
}

// ListRepositories returns all repositories in the index
func (m *Manager) ListRepositories() ([]RepoInfo, error) {
	rows, err := m.db.Query(`
		SELECT name, url, COALESCE(build, '') as build, COALESCE(executable, '') as executable, source_file, source_name, COALESCE(load, '') as load
		FROM repositories
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}
	defer rows.Close()

	var repos []RepoInfo
	for rows.Next() {
		var repo RepoInfo
		err := rows.Scan(
			&repo.Name,
			&repo.URL,
			&repo.Build,
			&repo.Executable,
			&repo.SourceFile,
			&repo.SourceName,
			&repo.Load,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan repository row: %w", err)
		}
		repos = append(repos, repo)
	}

	return repos, rows.Err()
}

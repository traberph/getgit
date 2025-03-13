package sources

import (
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// initDB creates the necessary database tables if they don't exist
func (sm *SourceManager) initDB() error {
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

	_, err := sm.db.Exec(schema)
	return err
}

// UpdateIndex updates the index database with the latest source information
func (sm *SourceManager) UpdateIndex() error {
	tx, err := sm.db.Begin()
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

	for _, source := range sm.Sources {
		s, ok := source.(*Source)
		if !ok {
			continue
		}
		for _, repo := range s.GetRepos() {
			_, err := stmt.Exec(
				repo.Name,
				repo.URL,
				repo.Build,
				repo.Executable,
				s.GetFilePath(),
				s.GetName(),
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
func (sm *SourceManager) FindRepository(name string) ([]RepoInfo, error) {
	rows, err := sm.db.Query(`
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

// ListRepositories returns all repositories in the index
func (sm *SourceManager) ListRepositories() ([]RepoInfo, error) {
	rows, err := sm.db.Query(`
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

// Close closes the database connection
func (sm *SourceManager) Close() error {
	return sm.db.Close()
}

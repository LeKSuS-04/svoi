package db

import (
	"database/sql"
	"fmt"

	_ "embed"

	_ "modernc.org/sqlite"

	"github.com/LeKSuS-04/svoi-bot/internal/db/q"
)

const InMemory = ":memory:"

//go:embed schema.sql
var ddl string

func New(dbPath string) (*q.Queries, error) {
	var connectionString string
	if dbPath == InMemory {
		connectionString = dbPath
	} else {
		connectionString = fmt.Sprintf("file://%s?mode=rwc&cache=shared&_journal_mode=WAL", dbPath)
	}

	db, err := sql.Open("sqlite", connectionString)
	if err != nil {
		return nil, fmt.Errorf("open db file: %w", err)
	}

	_, err = db.Exec(ddl)
	if err != nil {
		return nil, fmt.Errorf("init db: %w", err)
	}

	queries := q.New(db)
	return queries, nil
}

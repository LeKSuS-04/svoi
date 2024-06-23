package db

import (
	"database/sql"
	"fmt"

	_ "embed"

	_ "modernc.org/sqlite"

	"github.com/LeKSuS-04/svoi-bot/internal/db/q"
)

//go:embed schema.sql
var ddl string

func New(dbPath string) (*q.Queries, error) {
	db, err := sql.Open("sqlite", dbPath)
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

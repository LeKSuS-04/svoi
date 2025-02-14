package db

import (
	"fmt"
)

type Connector interface {
	Connect() (*DB, error)
}

type SimpleConnector struct {
	DbPath string
}

func (c *SimpleConnector) Connect() (*DB, error) {
	db, err := openConnection(c.DbPath)
	if err != nil {
		return nil, fmt.Errorf("open connection: %w", err)
	}
	return db, nil
}

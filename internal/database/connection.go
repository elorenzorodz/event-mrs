package database

import (
	"database/sql"
	"fmt"
)

func OpenConnection(dbURL string) (*sql.DB, error) {
	connection, connectionError := sql.Open("postgres", dbURL)

	if connectionError != nil {
		return nil, fmt.Errorf("could not open connection: %w", connectionError)
	}

	if pingError := connection.Ping(); pingError != nil {
		connection.Close()
		return nil, fmt.Errorf("could not ping database: %w", pingError)
	}

	return connection, nil
}
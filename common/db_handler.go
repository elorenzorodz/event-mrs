package common

import (
	"database/sql"
	"fmt"
	"log"
	"os"
)

func GetDBConnectionSettings() string {
	dbName := os.Getenv("DB_NAME")
	dbUrl := os.Getenv("DB_URL")

	return fmt.Sprintf(dbUrl, dbName)
}

func OpenDBConnection(dbUrl string) *sql.DB {
	connection, connectionError := sql.Open("postgres", dbUrl)

	if connectionError != nil {
		log.Fatal("Can't connect to database:", connectionError)
	}

	return connection
}
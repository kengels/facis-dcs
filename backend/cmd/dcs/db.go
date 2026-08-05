package main

import (
	"errors"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"log"
	"os"
)

func NewDatabaseConnection() (*sqlx.DB, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, errors.New("DATABASE_URL isn't set")
	}

	db, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		log.Fatalln(err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return db, nil
}

package database

import (
	"database/sql"
	"log"

	_ "github.com/lib/pq"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"

	_ "github.com/golang-migrate/migrate/v4/source/file"
)

var Db *sql.DB

func InitDB() {
	db, err := sql.Open("postgres", "postgres://postgres:dbpass@localhost:5432/btp_tokens?sslmode=disable")
	if err != nil {
		log.Panic(err)
	}

	if err = db.Ping(); err != nil {
		log.Panic(err)
	}
	Db = db
}

func CloseDB() error {
	return Db.Close()
}

func Migrate(path string) {
	if err := Db.Ping(); err != nil {
		log.Fatal(err)
	}
	driver, err := postgres.WithInstance(Db, &postgres.Config{}) 

	if err != nil {
		log.Fatal("failed to create migrate driver:", err)
	}

	
	m, err := migrate.NewWithDatabaseInstance(
		"file://" + path, 
		"postgres",
		driver,
	)
	if err != nil {
		log.Fatal("failed to create migrate instance:", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatal(err)
	}
}
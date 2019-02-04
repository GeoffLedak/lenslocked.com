package main

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

const (
	host   = "localhost"
	port   = 5432
	user   = "postgres"
	dbname = "lenslocked_dev"
)

func main() {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"dbname=%s sslmode=disable",
		host, port, user, dbname)
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}
	err = db.Ping()
	if err != nil {
		panic(err)
	}

	// The first version w/out the ID
	_, err = db.Exec(`
	  INSERT INTO users(name, email)
	  VALUES($1, $2)`,
		"Jon Calhoun", "jon@calhoun.io")
	if err != nil {
		panic(err)
	}

	// The second version that returns the ID
	var id int
	row := db.QueryRow(`
		INSERT INTO users(name, email)
		VALUES($1, $2) RETURNING id`,
		"Jon2 Calhoun2", "jon2@calhoun2.io")
	err = row.Scan(&id)
	if err != nil {
		panic(err)
	}

	db.Close()
}

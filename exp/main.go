package main

import (
	"fmt"

	"lenslocked.com/models"

	_ "github.com/lib/pq"
)

const (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = "P12RaspberryJam"
	dbname   = "lenslocked_dev"
)

func main() {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	us, err := models.NewUserService(psqlInfo)
	if err != nil {
		panic(err)
	}
	defer us.Close()
	us.DestructiveReset()

	// This will error because you DO NOT have a user with
	// this ID, but we will create one soon.
	user, err := us.ByID(1)
	if err != nil {
		panic(err)
	}
	fmt.Println(user)
}

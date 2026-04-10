package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	db, err := sql.Open("postgres", "host=localhost user=postgres password=1 dbname=costrict_stat sslmode=disable")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec("ALTER TABLE user_groups ADD COLUMN IF NOT EXISTS org_name VARCHAR(200) DEFAULT '';")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Migration done")
}

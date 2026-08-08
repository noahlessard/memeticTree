package main

import (
	"database/sql"
	"fmt"
	"os"

	// the underscore import registers the driver with database/sql
	_ "github.com/mattn/go-sqlite3"
)

func main() {

	// open database connection (create if it does not exist)

	var firstStartup error
	_, firstStartup = os.Stat("./app.db")

	db, err := sql.Open("sqlite3", "./app.db")
	if err != nil {
		fmt.Println("Failed to open database")
	}

	// when main finishes, close db
	defer db.Close()

	// check the connection is working
	if err := db.Ping(); err != nil {
		fmt.Println("Failed to ping database")
	}

	// if there was an error trying to stat db, its the first time,
	// so seed the db
	if firstStartup != nil {

		if err := initDatabase(db); err != nil {
			fmt.Println("Failed to initialize schema:")
		}
		seedDB(db)
	}

	fmt.Println("Successfully connected to SQLite database")

	startWebserver(db)

}

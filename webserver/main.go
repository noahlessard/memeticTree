package main

import (
	"database/sql"
	"fmt"

	// the underscore import registers the driver with database/sql
	_ "github.com/mattn/go-sqlite3"
)

func main() {

	// open database connection (create if it does not exist)
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

	if err := initDatabase(db); err != nil {
		fmt.Println("Failed to initialize schema:")
	}

	fmt.Println("Successfully connected to SQLite database")

	var myNode = Node{
		Name:        "Claude is running the sink",
		Description: "This image is comparing claude to running the sink, how silly.",
	}

	createnode(db, myNode)

	startWebserver(db)

}

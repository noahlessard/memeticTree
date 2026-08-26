package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
)

func seedDB(db *sql.DB) {

	fmt.Println("Seeding DB...")

	_, err := db.Exec(`DELETE FROM nodes;`)
	if err != nil {
		fmt.Println("couldn't clear db")
	}

	_, err = db.Exec(`DELETE FROM sqlite_sequence WHERE name='nodes';`)
	if err != nil {
		fmt.Println("couldn't reset autoincrement:", err)
	}

	// open passwords file
	f, err := os.Open("passwords.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// extract lines
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fmt.Printf("Creating user: %s\n", scanner.Text())
		// parse out user / password
		array := strings.Split(scanner.Text(), ",")
		if err := CreateUser(db, array[0], array[1]); err != nil {
			fmt.Printf("couldn't seed moderator %s: %s\n", array[0], err)
		}
	}

	// Create nodes and capture their IDs
	node1 := Node{
		Name:        "Test LUCA",
		Description: "I Should have no parents but lots of offspring",
		ImagePath:   "/assets/cat.png",
		Parents:     []Node{},
		Children:    []Node{},
		Tags:        []string{"cat", "god"},
	}
	id1 := createnode(db, node1)

	node2 := Node{
		Name:        "Test Parent",
		Description: "I should have both children and parent",
		ImagePath:   "/assets/cool.avif",
		Parents:     []Node{getnode(db, id1)},
		Children:    []Node{},
		Tags:        []string{"notext"},
	}
	id2 := createnode(db, node2)

	node3 := Node{
		Name:        "Test Child",
		Description: "I have no children but 2 parents",
		ImagePath:   "/assets/tweet.jpg",
		Parents:     []Node{getnode(db, id2), getnode(db, id1)},
		Children:    []Node{},
		Tags:        []string{"twitter"},
	}
	createnode(db, node3)

}

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

func initDatabase(db *sql.DB) error {
	const schema = `
	CREATE TABLE IF NOT EXISTS nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL UNIQUE,
	imagepath TEXT NOT NULL UNIQUE,
    parents TEXT,
    children TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP);`

	_, err := db.Exec(schema)
	return err

}

func createnode(db *sql.DB, node Node) {
	query := `
        INSERT INTO nodes (name, description, imagepath, parents, children, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `

	now := time.Now()
	parentsJSON, _ := json.Marshal(node.Parents)
	childrenJSON, _ := json.Marshal(node.Children)

	_, err := db.Exec(query,
		node.Name,
		node.Description,
		node.ImagePath,
		string(parentsJSON),
		string(childrenJSON),
		now,
		now,
	)

	if err != nil {
		fmt.Println(err)
	}
}

func getnode(db *sql.DB, id int) Node {
	query := `
	    SELECT id, name, description, imagepath, parents, children, created_at, updated_at
	    FROM nodes
	    WHERE id = ?
	`

	var node Node
	var parentsJSON sql.NullString
	var childrenJSON sql.NullString

	err := db.QueryRow(query, id).Scan(
		&node.ID,
		&node.Name,
		&node.Description,
		&node.ImagePath,
		&parentsJSON,
		&childrenJSON,
		&node.CreatedAt,
		&node.UpdatedAt,
	)

	if err != nil {
		fmt.Println(err)
	} else {
		if parentsJSON.Valid {
			json.Unmarshal([]byte(parentsJSON.String), &node.Parents)
		}
		if childrenJSON.Valid {
			json.Unmarshal([]byte(childrenJSON.String), &node.Children)
		}
	}

	return node
}

func getnodeByName(db *sql.DB, searchTerm string) []Node {
	query := `
	    SELECT id, name, description, imagepath, parents, children, created_at, updated_at
	    FROM nodes
	    WHERE name LIKE ?
	`

	// get all rows that possibly match query
	// need the odd string building for the LIKE keyword
	rows, err := db.Query(query, "%"+searchTerm+"%")
	if err != nil {
		fmt.Println(err)
		return []Node{}
	}

	defer rows.Close()

	// put all the results into node array
	var nodes []Node
	for rows.Next() {
		var node Node
		var parentsJSON sql.NullString
		var childrenJSON sql.NullString

		err := rows.Scan(
			&node.ID,
			&node.Name,
			&node.Description,
			&node.ImagePath,
			&parentsJSON,
			&childrenJSON,
			&node.CreatedAt,
			&node.UpdatedAt,
		)

		if err != nil {
			fmt.Println(err)
			continue
		}

		if parentsJSON.Valid {
			json.Unmarshal([]byte(parentsJSON.String), &node.Parents)
		}
		if childrenJSON.Valid {
			json.Unmarshal([]byte(childrenJSON.String), &node.Children)
		}

		nodes = append(nodes, node)
	}

	return nodes
}

func countNodes(db *sql.DB) int {
	query := `SELECT COUNT(*) FROM nodes`

	var count int
	err := db.QueryRow(query).Scan(&count)

	if err != nil {
		fmt.Println(err)
		return -1
	}

	return count
}

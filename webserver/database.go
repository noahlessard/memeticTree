package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

func initDatabase(db *sql.DB) error {

	// TODO: I'm not sure if these not nulls are actually enforced
	// with how go defaults things. Double check eventually
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

func createnode(db *sql.DB, node Node) int {

	query := `
        INSERT INTO nodes (name, description, imagepath, parents, children, created_at, updated_at)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `

	now := time.Now()
	parentsJSON, _ := json.Marshal(node.Parents)
	childrenJSON, _ := json.Marshal(node.Children)

	result, err := db.Exec(query,
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

	id, err := result.LastInsertId()
	if err != nil {
		fmt.Println("Error getting ID:", err)
		return -1
	}

	// update node data structure before we check updating relatives
	node.ID = int(id)

	// check if we need to update parents / children
	updateRelatives(db, node)

	return int(id)

}

func updateRelatives(db *sql.DB, node Node) {

	if len(node.Children) > 0 {
		for _, childNode := range node.Children {
			// update the child nodes to have this node as its parent
			fmt.Println(childNode)
			updateParents(db, childNode.ID, node)
		}
	}

	if len(node.Parents) > 0 {

		for _, parentNode := range node.Parents {
			// update the parent nodes to have this node as a child
			updateChildren(db, parentNode.ID, node)
		}
	}
}

func updateParents(db *sql.DB, changedNode int, newNode Node) error {

	// Get the changing node, so we can make sure its not duped
	changedNodeDB := getnode(db, changedNode)

	for _, p := range changedNodeDB.Parents {
		if p.ID == newNode.ID {
			return nil
		}
	}

	// Add the parent
	changedNodeDB.Parents = append(changedNodeDB.Parents, newNode)

	// Marshal back to JSON and update database
	newJson, err := json.Marshal(changedNodeDB.Parents)
	if err != nil {
		return err
	}

	_, err = db.Exec(
		"UPDATE nodes SET parents = ? WHERE id = ?",
		string(newJson),
		changedNode,
	)
	return err
}

func updateChildren(db *sql.DB, changedNode int, newNode Node) error {
	// Get the changing node, so we can make sure its not duped
	changedNodeDB := getnode(db, changedNode)

	// Check if child already exists to avoid duplicates
	for _, c := range changedNodeDB.Children {
		if c.ID == newNode.ID {
			return nil
		}
	}

	// Add the child
	changedNodeDB.Children = append(changedNodeDB.Children, newNode)

	// Marshal back to JSON and update database
	childrenJSON, err := json.Marshal(changedNodeDB.Children)
	if err != nil {
		return err
	}

	_, err = db.Exec(
		"UPDATE nodes SET children = ? WHERE id = ?",
		string(childrenJSON),
		changedNode,
	)
	return err
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

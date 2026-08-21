package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP);
	
	
	CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY,
    name TEXT UNIQUE NOT NULL);

	CREATE TABLE IF NOT EXISTS junction_node_tags (
    item_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    PRIMARY KEY (item_id, tag_id),
    FOREIGN KEY (item_id) REFERENCES nodes(id),
    FOREIGN KEY (tag_id) REFERENCES tags(id));
	
	
	CREATE TABLE IF NOT EXISTS mods (
	id INTEGER PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	hash TEXT NOT NULL UNIQUE)
	`

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

	// Add tags if provided
	if len(node.Tags) > 0 {
		err := addTagsToNode(db, node.ID, node.Tags)
		if err != nil {
			fmt.Println("error adding tags:", err)
		}
	}

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

func unwrapSQLNodes(rows *sql.Rows) []Node {

	// put all the results into node array
	var returnNodes []Node
	for rows.Next() {
		var node Node
		var parentsJSON sql.NullString
		var childrenJSON sql.NullString
		var tagsStr sql.NullString

		err := rows.Scan(
			&node.ID,
			&node.Name,
			&node.Description,
			&node.ImagePath,
			&parentsJSON,
			&childrenJSON,
			&node.CreatedAt,
			&node.UpdatedAt,
			&tagsStr,
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

		// parse tags from comma-separated string
		if tagsStr.Valid && tagsStr.String != "" {
			node.Tags = strings.Split(tagsStr.String, ",")
		} else {
			node.Tags = []string{}
		}

		returnNodes = append(returnNodes, node)
	}

	return returnNodes

}

func getnode(db *sql.DB, id int) Node {
	query := `
		SELECT 
			n.id, 
			n.name, 
			n.description, 
			n.imagepath, 
			n.parents, 
			n.children, 
			n.created_at, 
			n.updated_at,
			COALESCE(GROUP_CONCAT(t.name, ','), '') as tags
		FROM nodes n
		LEFT JOIN junction_node_tags nt ON n.id = nt.item_id
		LEFT JOIN tags t ON nt.tag_id = t.id
		WHERE n.id = ?
		GROUP BY n.id
	`

	rows, err := db.Query(query, id)
	if err != nil {
		fmt.Println(err)
		return Node{}
	}
	defer rows.Close()

	return unwrapSQLNodes(rows)[0]
}

func getnodeByName(db *sql.DB, searchTerm string) []Node {
	query := `
		SELECT 
			n.id, 
			n.name, 
			n.description, 
			n.imagepath, 
			n.parents, 
			n.children, 
			n.created_at, 
			n.updated_at,
			GROUP_CONCAT(t.name, ',') as tags
		FROM nodes n
		LEFT JOIN junction_node_tags nt ON n.id = nt.item_id
		LEFT JOIN tags t ON nt.tag_id = t.id
		WHERE n.name LIKE ?
		GROUP BY n.id
		ORDER BY n.name
	`

	// get all rows that possibly match query
	// need the odd string building for the LIKE keyword
	rows, err := db.Query(query, "%"+searchTerm+"%")
	if err != nil {
		fmt.Println(err)
		return []Node{}
	}

	defer rows.Close()

	return unwrapSQLNodes(rows)
}

func getnodeByTags(db *sql.DB, tags []string) []Node {
	placeholders := strings.Repeat("?,", len(tags)-1) + "?"

	query := fmt.Sprintf(`
		SELECT DISTINCT
			i.id, 
			i.name, 
			i.description, 
			i.imagepath, 
			i.parents, 
			i.children, 
			i.created_at, 
			i.updated_at,
			COALESCE(GROUP_CONCAT(t.name, ','), '') as tags
		FROM nodes i
		JOIN junction_node_tags it ON i.id = it.item_id
		JOIN tags t ON it.tag_id = t.id
		WHERE t.name IN (%s)
		GROUP BY i.id`, placeholders)

	// Convert []string to []interface{} for db.Query
	args := make([]interface{}, len(tags))
	for i, tag := range tags {
		args[i] = tag
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		fmt.Println(err)
		return []Node{}
	}

	defer rows.Close()

	return unwrapSQLNodes(rows)

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

func addTagsToNode(db *sql.DB, nodeID int, tags []string) error {
	for _, tagName := range tags {
		// Insert tag if it doesn't exist
		_, err := db.Exec("INSERT OR IGNORE INTO tags (name) VALUES (?)", tagName)
		if err != nil {
			return err
		}

		// Get the tag ID
		var tagID int
		err = db.QueryRow("SELECT id FROM tags WHERE name = ?", tagName).Scan(&tagID)
		if err != nil {
			return err
		}

		// Insert into junction table (ignore if already linked)
		_, err = db.Exec(
			"INSERT OR IGNORE INTO junction_node_tags (item_id, tag_id) VALUES (?, ?)",
			nodeID, tagID,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func removeTagsFromNode(db *sql.DB, nodeID int, tags []string) error {
	for _, tagName := range tags {
		var tagID int
		err := db.QueryRow("SELECT id FROM tags WHERE name = ?", tagName).Scan(&tagID)
		if err != nil {
			continue // Tag doesn't exist, skip
		}

		_, err = db.Exec(
			"DELETE FROM junction_node_tags WHERE item_id = ? AND tag_id = ?",
			nodeID, tagID,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

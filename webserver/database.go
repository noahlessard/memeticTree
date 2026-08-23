package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func initDatabase(db *sql.DB) error {

	// sqlite enforces NOT NULL at the schema level regardless of how
	// the driver defaults values, so no code-side guard is needed.
	const schema = `
	CREATE TABLE IF NOT EXISTS nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL UNIQUE,
	imagepath TEXT NOT NULL UNIQUE,
    parents TEXT,
    children TEXT);
	
	
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
	hash TEXT NOT NULL UNIQUE);
	
	CREATE TABLE IF NOT EXISTS submissions (
	id INTEGER PRIMARY KEY,
	name TEXT,
	description TEXT,
	imagepath TEXT NOT NULL UNIQUE,
	parent TEXT,
	child TEXT,
	potentialtags TEXT)`

	_, err := db.Exec(schema)
	return err

}

func createnode(db *sql.DB, node Node) int {

	query := `
        INSERT INTO nodes (name, description, imagepath, parents, children)
        VALUES (?, ?, ?, ?, ?)
    `

	parentsJSON, _ := json.Marshal(node.Parents)
	childrenJSON, _ := json.Marshal(node.Children)

	result, err := db.Exec(query,
		node.Name,
		node.Description,
		node.ImagePath,
		string(parentsJSON),
		string(childrenJSON),
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
			&tagsStr,
		)

		if err != nil {
			fmt.Println(err)
			continue
		}

		// TODO: check if these nodes actually exist?
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

	nodeArray := unwrapSQLNodes(rows)
	if len(nodeArray) > 0 {
		return nodeArray[0]
	} else {
		return Node{}
	}
}

func getSubmission(db *sql.DB, id int) Node {
	query := `
		SELECT 
			id, 
			name, 
			description, 
			imagepath, 
			parent, 
			child, 
			potentialtags
		FROM submissions
		WHERE id = ?
	`

	rows, err := db.Query(query, id)
	if err != nil {
		fmt.Println(err)
		return Node{}
	}
	defer rows.Close()
	nodeArray := unwrapSQLNodes(rows)
	if len(nodeArray) > 0 {
		return nodeArray[0]
	} else {
		return Node{}
	}

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
			GROUP_CONCAT(t.name, ',') as tags
		FROM nodes n
		LEFT JOIN junction_node_tags nt ON n.id = nt.item_id
		LEFT JOIN tags t ON nt.tag_id = t.id
		WHERE n.name LIKE ?
		GROUP BY n.id
		ORDER BY n.name
	`

	// get all rows that possibly match query
	rows, err := db.Query(query, "%"+searchTerm+"%")
	if err != nil {
		fmt.Println(err)
		return []Node{}
	}

	defer rows.Close()

	return unwrapSQLNodes(rows)
}

// check if you got an empty node by checking if ID == 0 (un-init)
func getnodeByNameExact(db *sql.DB, searchTerm string) Node {
	query := `
		SELECT 
			n.id, 
			n.name, 
			n.description, 
			n.imagepath, 
			n.parents, 
			n.children, 
			GROUP_CONCAT(t.name, ',') as tags
		FROM nodes n
		LEFT JOIN junction_node_tags nt ON n.id = nt.item_id
		LEFT JOIN tags t ON nt.tag_id = t.id
		WHERE n.name = ?
		GROUP BY n.id
		ORDER BY n.name
	`

	// get the node that exactly matches
	rows, err := db.Query(query, searchTerm)
	if err != nil {
		fmt.Println(err)
		return Node{}
	}

	defer rows.Close()

	rowsArray := unwrapSQLNodes(rows)
	// if there is more than one node, return nothing (how did this even happen?)
	if len(rowsArray) != 1 {
		return Node{}
	} else {
		return rowsArray[0]
	}

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

func updateNode(db *sql.DB, node Node) error {

	parentsJSON, _ := json.Marshal(node.Parents)
	childrenJSON, _ := json.Marshal(node.Children)

	_, err := db.Exec(`UPDATE nodes SET name = ?, description = ?, imagepath = ?, parents = ?, children = ? WHERE id = ?`,
		node.Name,
		node.Description,
		node.ImagePath,
		string(parentsJSON),
		string(childrenJSON),
		node.ID)
	if err != nil {
		return err
	}

	// replace the tag links with the input node's tags
	_, err = db.Exec("DELETE FROM junction_node_tags WHERE item_id = ?", node.ID)
	if err != nil {
		return err
	}
	return addTagsToNode(db, node.ID, node.Tags)
}

func updateSubmission(db *sql.DB, node Node) error {

	parentsJSON, _ := json.Marshal(node.Parents)
	childrenJSON, _ := json.Marshal(node.Children)

	// tags are stored as a comma-joined string in the submissions table
	var tagString string
	if len(node.Tags) > 0 {
		tagString = strings.Join(node.Tags, ", ")
	}

	_, err := db.Exec(`UPDATE submissions SET name = ?, description = ?, imagepath = ?, parent = ?, child = ?, potentialtags = ? WHERE id = ?`,
		node.Name,
		node.Description,
		node.ImagePath,
		string(parentsJSON),
		string(childrenJSON),
		tagString,
		node.ID)
	return err
}

// CreateUser upserts a moderator, storing a bcrypt hash of the password.
// INSERT OR REPLACE keeps re-seeding idempotent so env changes apply.
func CreateUser(db *sql.DB, name, password string) error {
	if name == "" || password == "" {
		return errors.New("Error: username and password can't be blank")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO mods (name, hash) VALUES (?, ?)`, name, string(hash))
	return err
}

func isMod(db *sql.DB, name, password string) bool {
	var hash string
	err := db.QueryRow(`SELECT hash FROM mods WHERE name = ?`, name).Scan(&hash)
	if err != nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func insertSubmission(db *sql.DB, inputSubmission Node) error {

	// check that submission atleast has an image path (everything else can be empty)
	if len(inputSubmission.ImagePath) <= 0 {
		return errors.New("image path cannot be null when submitting")
	}

	// can't store tags as array, so use comma sep string
	// we will unmarshall and insert tags in the moderation accept code
	var tagString string
	if len(inputSubmission.Tags) > 0 {
		tagString = strings.Join(inputSubmission.Tags, ", ")
	}

	parentsJSON, _ := json.Marshal(inputSubmission.Parents)
	childrenJSON, _ := json.Marshal(inputSubmission.Children)

	_, err := db.Exec(`INSERT INTO submissions (name, description, imagepath, parent, child, potentialtags) VALUES (?, ?, ?, ?, ?, ?)`,
		inputSubmission.Name,
		inputSubmission.Description,
		inputSubmission.ImagePath,
		string(parentsJSON),
		string(childrenJSON),
		tagString)

	if err != nil {
		return errors.New("Error: unable to insert submission into database")
	}

	return nil

}

func getSubmissions(db *sql.DB, offset int, pageSize int) []Node {
	query := `
		SELECT id, name, description, imagepath, parent, child, potentialtags
		FROM submissions
		ORDER BY id
		LIMIT ? OFFSET ?
	`

	rows, err := db.Query(query, pageSize, offset)
	if err != nil {
		return []Node{}
	}
	defer rows.Close()

	return unwrapSQLNodes(rows)
}

func removeSubmission(db *sql.DB, submissionNode Node) error {

	fileName, found := strings.CutPrefix(submissionNode.ImagePath, "/assets/")

	// if the image path passed in had assets, change to ../uploads
	if found == true {
		fileName = "../uploads/" + fileName
	} else {
		// it was called already with ../uploads, so its fine
	}

	err := os.Remove(fileName)
	if err != nil {
		log.Fatal(err)
	}

	query := `
		DELETE FROM submissions WHERE id = ?
	`
	_, err = db.Exec(query, submissionNode.ID)

	if err != nil {
		return err
	} else {
		return nil
	}

}

func moveSubmissionToNodes(db *sql.DB, submissionNode Node) error {

	// first move image from uploads to assets
	bytesRead, err := os.ReadFile(submissionNode.ImagePath)
	if err != nil {
		log.Fatal(err)
	}
	fileName, _ := strings.CutPrefix(submissionNode.ImagePath, "../uploads/")
	err = os.WriteFile("./assets/"+fileName, bytesRead, 0644)
	if err != nil {
		log.Fatal(err)
	}

	// save this under slightly different path for http handling
	submissionNode.ImagePath = "/assets/" + fileName
	createnode(db, submissionNode)

	removeSubmission(db, submissionNode)

	return nil
}

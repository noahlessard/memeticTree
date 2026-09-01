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
		Name:        "Glen Powell's October 2025 GQ Magazine shoot",
		Description: "In this photo shoot, Glen Powell posed as stereotypical portraits of American life. One of these was an American politician.",
		ImagePath:   "/assets/original-paul-e-tician.avif",
		Parents:     []Node{},
		Children:    []Node{},
		Tags:        []string{"Glen Powell", "Paul E Tician", "John Politics"},
	}
	id1 := createnode(db, node1)

	node2 := Node{
		Name:        "John Politics",
		Description: "This tweet appears to be the first reference to Glen Powell as 'John Politics', and significantly, also points out Glen Powell's glossy nature.",
		ImagePath:   "/assets/the-john-politics-paul-e-tician-v0.avif",
		Parents:     []Node{getnode(db, id1)},
		Children:    []Node{},
		Tags:        []string{"Glen Powell", "Paul E Tician", "John Politics", "Twitter"},
	}
	id2 := createnode(db, node2)

	node3 := Node{
		Name:        "Glen Powell White Pharaoh",
		Description: "An alternative meme made about Glen Powell's politician photo shoot, that didn't adopt any form of glossiness, instead mutating the previous 'white pharaoh' meme.",
		ImagePath:   "/assets/paul-e-tician-white-pharoh.avif",
		Parents:     []Node{getnode(db, id1)},
		Children:    []Node{},
		Tags:        []string{"Glen Powell", "White Pharaoh"},
	}
	createnode(db, node3)

	node4 := Node{
		Name:        "Glossy Paul E. Tician",
		Description: "This is an example of the early variants of John Politics that began proliferating rapidly across many different social media platforms. It appears that around this time is also when Paul E. Tician was assigned to the character.",
		ImagePath:   "/assets/glossy.avif",
		Parents:     []Node{getnode(db, id2)},
		Children:    []Node{},
		Tags:        []string{"Glen Powell", "Paul E Tician", "John Politics", "Glossy", "Oily"},
	}
	id4 := createnode(db, node4)

	node5 := Node{
		Name:        "Chrome Paul E. Tician",
		Description: "This meme exaggerates the gloss of previous versions to the point where the character appears to be made of chrome, instead of being oily. This might have originated through the stacking of filters, although AI generated imagery was probably used as well. The addition of white monster also indicates a possible overlap with the hyper exaggeration of vril aesthetics.",
		ImagePath:   "/assets/paul-e-tician-chrome.jpeg.avif",
		Parents:     []Node{getnode(db, id4)},
		Children:    []Node{},
		Tags:        []string{"Paul E Tician", "John Politics", "Chrome", "White Monster"},
	}
	id5 := createnode(db, node5)

	node8 := Node{
		Name:        "Golden Paul E. Tician",
		Description: "This meme represents an interesting dead end in the Paul E Tician line. Although it follows the same path as the Chrome line, carrying the traits of  AI generated edits and absurdity, this golden variant is much less popular.",
		ImagePath:   "/assets/goldengloss.avif",
		Parents:     []Node{getnode(db, id4)},
		Children:    []Node{},
		Tags:        []string{"Paul E Tician", "John Politics", "Golden"},
	}
	createnode(db, node8)

	node6 := Node{
		Name:        "Chromeposure Paul E. Tician",
		Description: "A spinoff of the Chrome Paul E. Tician variant, showings its popularity and versatility. This addition adds the 'chromeposture' keyword, used to indicate the characters stereotypical politician confidence (in an absurdist way).",
		ImagePath:   "/assets/chromeposture.avif",
		Parents:     []Node{getnode(db, id5)},
		Children:    []Node{},
		Tags:        []string{"Paul E Tician", "John Politics", "Chrome", "Chromeposure"},
	}
	id6 := createnode(db, node6)

	node7 := Node{
		Name:        "Chromefortable Paul E. Tician",
		Description: "A spinoff of the Chrome Paul E. Tician 'chromeposure' variant, this meme carries the pun trait along with the continued aburdism. The original Glen Powell image is completely lost now, despite him reassuring us that this isn't even his final form.",
		ImagePath:   "/assets/finalform.avif",
		Parents:     []Node{getnode(db, id6)},
		Children:    []Node{},
		Tags:        []string{"Paul E Tician", "Chrome", "Chromeposure", "Chromefortable"},
	}
	createnode(db, node7)

}

package main

import (
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
)

var lastUpload sync.Map

const rateLimit = 30 * time.Second
const fileSize = 2 * 1024 * 1024
const memLimit = 5 * 1024 * 1024
const tagLimit = 100

var allowedSearches = [3]string{"none", "parent", "child"}

func startWebserver(db *sql.DB) {

	assetsFs := http.FileServer(http.Dir("assets"))
	http.Handle("/assets/", http.StripPrefix("/assets/", assetsFs))

	uploadsFs := http.FileServer(http.Dir("../uploads"))
	http.Handle("/uploads/", http.StripPrefix("/uploads/", uploadsFs))

	// serve html from templ
	http.Handle("/", templ.Handler(homepage(db)))
	http.Handle("/about", templ.Handler(about()))

	// server html from our custom functions
	http.HandleFunc("/subpage", makeSubpageHandler(db))
	http.HandleFunc("/editpage", requireAuth(makeEditpageHandler(db, false)))
	http.HandleFunc("/editsubmission", requireAuth(makeEditpageHandler(db, true)))
	http.HandleFunc("/submission", handleSubmission(db))
	http.HandleFunc("/search", makeHandleSearch(db))
	http.HandleFunc("/random", makeRandompageHandler(db))
	http.HandleFunc("/randomvisual", makeHandleRandomVisual(db))
	http.HandleFunc("/login", makeHandleLogin(db))
	http.HandleFunc("/logout", makeHandleLogout())
	http.HandleFunc("/moderator", requireAuth(makeModeration(db)))
	http.HandleFunc("/approve", requireAuth(handleApproval(db, true)))
	http.HandleFunc("/reject", requireAuth(handleApproval(db, false)))

	fmt.Println("Listening on :3000")
	http.ListenAndServe(":3000", nil)

}

func makeSubpageHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		nodeIDStr := r.URL.Query().Get("id")
		nodeID, err := strconv.Atoi(nodeIDStr)
		if err != nil || nodeID == 0 {
			http.Error(w, "Invalid node ID", http.StatusBadRequest)
			return
		}

		node := getnode(db, nodeID)

		_, modBool := checkSession(r)
		templ.Handler(subpage(node, modBool)).ServeHTTP(w, r)
	}
}

func makeEditpageHandler(db *sql.DB, submissionBool bool) authedHandler {
	return func(w http.ResponseWriter, r *http.Request, session modSession) {

		nodeIDStr := r.URL.Query().Get("id")
		nodeID, err := strconv.Atoi(nodeIDStr)
		if err != nil || nodeID == 0 {
			http.Error(w, "Invalid node ID", http.StatusBadRequest)
			return
		}

		var node Node
		if submissionBool == true {
			node = getSubmission(db, nodeID)
		} else {
			node = getnode(db, nodeID)
		}

		if r.Method == http.MethodPost {
			selectedName := strings.TrimSpace(r.FormValue("name"))
			selectedDescription := strings.TrimSpace(r.FormValue("description"))

			node.Name = selectedName
			node.Description = selectedDescription

			if submissionBool {
				err = updateSubmission(db, node)
			} else {
				err = updateNode(db, node)
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if submissionBool {
				http.Redirect(w, r, "/moderator", http.StatusSeeOther)
			} else {
				http.Redirect(w, r, "/subpage?id="+fmt.Sprint(nodeID), http.StatusSeeOther)
			}
			return
		}

		templ.Handler(editpage(node)).ServeHTTP(w, r)
	}
}

func makeRandompageHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var min = 1
		var max = countNodes(db)
		var randomNode = rand.Intn(max-min+1) + min
		node := getnode(db, randomNode)

		_, modBool := checkSession(r)
		templ.Handler(subpage(node, modBool)).ServeHTTP(w, r)
	}
}

// pick a random node with no parents (a LUCA) and render its children as a tree
func makeHandleRandomVisual(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var root Node
		for i := 0; i < 50; i++ {
			max := countNodes(db)
			if max <= 0 {
				break
			}
			node := getnode(db, rand.Intn(max)+1)
			if node.ID != 0 && len(node.Parents) == 0 {
				root = node
				break
			}
		}
		if root.ID == 0 {
			http.Error(w, "No LUCA found", http.StatusNotFound)
			return
		}

		tree := buildSubtree(db, root, map[int]bool{})
		templ.Handler(randomvisual(tree)).ServeHTTP(w, r)
	}
}

// recursively expands the input node by re-fetching each child from the DB
// doesn't shown any "upstream" nodes, only downstream
func buildSubtree(db *sql.DB, node Node, expanded map[int]bool) Node {
	if expanded[node.ID] {
		return Node{ID: node.ID, Name: node.Name}
	}
	expanded[node.ID] = true

	full := getnode(db, node.ID)
	children := make([]Node, 0, len(full.Children))
	for _, c := range full.Children {
		children = append(children, buildSubtree(db, c, expanded))
	}
	full.Children = children
	return full
}

// TODO: Limit the amount of search results returned, pagination?
func makeHandleSearch(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var results []Node
		var searchType string
		var searchQuery string

		// this structure either gets the post value (button press)
		// or falls back to the query value if not given
		searchQuery = r.FormValue("nodeinput")
		if searchQuery == "" {
			searchQuery = r.URL.Query().Get("query")
		}

		searchType = r.FormValue("searchType")
		if searchType == "" {
			searchType = r.URL.Query().Get("searchType")
		}

		// if search query exists, search for results
		// TODO: add a pagination here?
		if searchQuery != "" {
			if searchType == "name" {
				results = getnodeByName(db, searchQuery)
				// this else if check if redundant... for now
			} else if searchType == "tags" {
				tagSplit := strings.Split(searchQuery, ",")
				tags := make([]string, len(tagSplit))
				for i, tag := range tagSplit {
					tags[i] = strings.TrimSpace(tag)
				}
				results = getnodeByTags(db, tags)
			}
		}

		// render the search component with or without results, type, query
		templ.Handler(search(results, searchType, searchQuery)).ServeHTTP(w, r)
	}
}

func makeHandleLogin(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {

			// check that csrf is valid
			c, _ := r.Cookie(csrfName)
			if len(c.Value) <= 0 &&
				subtle.ConstantTimeCompare([]byte(c.Value), []byte(r.FormValue("csrf"))) != 1 {
				http.Error(w, "invalid csrf token", http.StatusForbidden)
				return
			}

			// verify login-form csrf (defense in depth on top of SameSite)
			// check that it matches what the form said
			name := r.FormValue("name")
			if isMod(db, name, r.FormValue("password")) {
				// create the CSRF side of the token
				csrf, err := newToken()
				if err != nil {
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}
				// this session is good, create and store it
				session := modSession{user: name, csrf: csrf, expires: time.Now().Add(sessionTTL)}
				if err := setSession(w, r, session); err != nil {
					http.Error(w, "internal error", http.StatusInternalServerError)
					return
				}
				http.Redirect(w, r, "/moderator", http.StatusSeeOther)
				return
			}

			// bad creds: re-render the form, keep the csrf cookie valid
			templ.Handler(login(getOrCreateCSRF(w, r), "Invalid username or password")).ServeHTTP(w, r)
			return
		}

		templ.Handler(login(getOrCreateCSRF(w, r), "")).ServeHTTP(w, r)
	}
}

func makeHandleLogout() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clearSession(w, r)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

// authedHandler is an http.HandlerFunc that also receives the validated session.
// requiring this signature means a handler can't be registered without requireAuth!
type authedHandler func(w http.ResponseWriter, r *http.Request, session modSession)

func makeModeration(db *sql.DB) authedHandler {
	return func(w http.ResponseWriter, r *http.Request, session modSession) {
		submissions := getSubmissions(db, 0, 100)
		templ.Handler(modpanel(db, session.user, submissions)).ServeHTTP(w, r)
	}
}

func handleSubmission(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// set this to true when we submit okay
		status := false

		// this is used to name this submission
		selectedName := strings.TrimSpace(r.FormValue("nameinput"))
		if selectedName == "" {
			selectedName = r.URL.Query().Get("nameinput")
		}
		selectedDescription := strings.TrimSpace(r.FormValue("descriptioninput"))
		if selectedDescription == "" {
			selectedDescription = r.URL.Query().Get("descriptioninput")
		}
		// can be none, parent, or child (see above consts)
		selectedType := strings.TrimSpace(r.FormValue("searchType"))
		if selectedType == "" {
			selectedType = r.URL.Query().Get("searchType")
		}
		// this is used to enter the name of referenced node (parent / child / none)
		selectedReference := strings.TrimSpace(r.FormValue("reference"))
		if selectedReference == "" {
			selectedReference = r.URL.Query().Get("reference")
		}
		selectedTagString := strings.TrimSpace(r.FormValue("tagsinput"))
		if selectedTagString == "" {
			selectedTagString = r.URL.Query().Get("tagsinput")
		}

		if r.Method == "GET" {
			templ.Handler(submission(selectedName, selectedDescription, selectedTagString, selectedType, selectedReference, status)).ServeHTTP(w, r)
			return
		} else if r.Method == "POST" {

			// bound the body before anything reads it
			// this will return a hard error but good safety stop
			r.Body = http.MaxBytesReader(w, r.Body, fileSize)
			if err := r.ParseMultipartForm(memLimit); err != nil {
				http.Error(w, "request too large to parse!", http.StatusBadRequest)
				return
			}

			// validate strings, looking up name as well to verify it matches
			cleanTags, err := returnCleanSubmissionFields(db, selectedName, selectedTagString, selectedType)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			// parse out file now that we know its under size
			file, header, err := r.FormFile("image")
			if err != nil {
				http.Error(w, "Error: No file uploaded", http.StatusBadRequest)
				return
			}
			defer file.Close()

			// checker header type anyways, just in case it prevent expensive file ops
			// gets content type as 'image/*format*/
			clientContentType := header.Header.Get("Content-Type")
			foundHeader := false
			for _, item := range allowedTypes {
				if clientContentType == item {
					foundHeader = true
					break
				}
			}
			if foundHeader == false {
				http.Error(w, "Error: incorrect content type", http.StatusBadRequest)
				return
			}

			// check actual file dimensions, type, contents
			// filetype is returned as 'image/*format*'
			err = safeCheckContents(file)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			// actually save image
			err, filepath := safeSaveFile(file)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			// don't set ID here, database code handles it
			submissionNode := Node{
				Name:        selectedName,
				Description: selectedDescription,
				ImagePath:   filepath,
				Tags:        cleanTags,
			}

			// check that reference node exists, then add it to parent or child field
			if len(selectedReference) > 0 {
				referencedNode := getnodeByNameExact(db, selectedReference)
				if referencedNode.ID == 0 {
					http.Error(w, "Error: can't find reference node. Match must be exact", http.StatusBadRequest)
					return
				}
				if selectedType == "child" {
					submissionNode.Parents = []Node{referencedNode}
				} else if selectedType == "parent" {
					submissionNode.Children = []Node{referencedNode}
				} else if selectedType == "none" {
					http.Error(w, "Error: can't include a reference without specifying type", http.StatusBadRequest)
					return
				}
			}

			// insertSubmission uses parameterized queries, so no injection risk here
			err = insertSubmission(db, submissionNode)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			status = true

			// clear tags when submission is successful
			templ.Handler(submission("", "", "", "", "", status)).ServeHTTP(w, r)
			return
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}
}

// putting some extra parsing code here... maybe overkill
// will return the tags field as an array instead of just a string
// these don't need to check the size bc go already limit get size reasonably
func returnCleanSubmissionFields(db *sql.DB, name string, tags string, searchtype string) ([]string, error) {

	// check tag size
	tagSplit := strings.Split(tags, ",")
	if len(tagSplit) > tagLimit {
		return []string{}, errors.New("Error: too many tags submitted")
	}

	// check searchtype is valid (code injection?)
	if slices.Contains(allowedSearches[:], searchtype) != true {
		return []string{}, errors.New("Error: no valid search type submitted")
	}

	// build and return tag array
	tagsArr := make([]string, len(tagSplit))
	for i, tag := range tagSplit {
		tagsArr[i] = strings.TrimSpace(tag)
	}

	return tagsArr, nil

}

func handleApproval(db *sql.DB, status bool) authedHandler {
	return func(w http.ResponseWriter, r *http.Request, session modSession) {
		selectedSubmisisonString := r.URL.Query().Get("id")
		selectedSubmisison, _ := strconv.Atoi(selectedSubmisisonString)
		submissionNode := getSubmission(db, selectedSubmisison)
		if submissionNode.ID == 0 {
			http.Error(w, "Error: Could not find that submission to approve", http.StatusBadRequest)
			return
		}
		if status == true {
			// run database function to move submission from sub table to
			moveSubmissionToNodes(db, submissionNode)
		} else {
			removeSubmission(db, submissionNode)
		}

		// then recall the makeModeration function
		http.Redirect(w, r, "/moderator", http.StatusSeeOther)
	}
}

// wrap our http handler functions with this to require a csrf
// and redirect to login if one is not found
func requireAuth(next authedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := checkSession(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next(w, r, session)
	}
}

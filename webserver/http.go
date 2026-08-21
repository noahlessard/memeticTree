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

	// serve the images from disk for now
	assetsDir := http.Dir(".")
	fs := http.FileServer(assetsDir)
	http.Handle("/assets/", fs)

	// serve html from templ
	http.Handle("/", templ.Handler(homepage(db)))
	http.Handle("/about", templ.Handler(about()))

	// server html from our custom functions
	http.HandleFunc("/subpage", makeSubpageHandler(db))
	http.HandleFunc("/submission", handleSubmission(db))
	http.HandleFunc("/search", makeHandleSearch(db))
	http.HandleFunc("/random", makeRandompageHandler(db))
	http.HandleFunc("/login", makeHandleLogin(db))
	http.HandleFunc("/logout", makeHandleLogout())
	http.HandleFunc("/moderator", requireAuth(makeModeration()))

	fmt.Println("Listening on :3000")
	http.ListenAndServe(":3000", nil)

}

func makeSubpageHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		nodeIDStr := r.URL.Query().Get("id")
		nodeID, err := strconv.Atoi(nodeIDStr)
		if err != nil {
			http.Error(w, "Invalid node ID", http.StatusBadRequest)
			return
		}

		node := getnode(db, nodeID)

		templ.Handler(subpage(node)).ServeHTTP(w, r)
	}
}

func makeRandompageHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var min = 1
		var max = countNodes(db)
		var randomNode = rand.Intn(max-min+1) + min
		node := getnode(db, randomNode)
		templ.Handler(subpage(node)).ServeHTTP(w, r)
	}
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

func makeModeration() authedHandler {
	return func(w http.ResponseWriter, r *http.Request, session modSession) {
		templ.Handler(modpanel(session.user)).ServeHTTP(w, r)
	}
}

func handleSubmission(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var selectedString string
		var selectedTagString string
		var selectedType string
		// set this to true when we submit okay
		status := false

		// always get this items regardless
		selectedString = strings.TrimSpace(r.FormValue("nameinput"))
		if selectedString == "" {
			selectedString = r.URL.Query().Get("nameinput")
		}
		// can be none, parent, or child (see above consts)
		selectedType = strings.TrimSpace(r.FormValue("searchType"))
		if selectedType == "" {
			selectedType = r.URL.Query().Get("searchType")
		}
		selectedTagString = strings.TrimSpace(r.FormValue("tagsinput"))
		if selectedTagString == "" {
			selectedTagString = r.URL.Query().Get("tagsinput")
		}

		if r.Method == "GET" {
			templ.Handler(submission(selectedString, selectedTagString, selectedType, status)).ServeHTTP(w, r)
			return
		} else if r.Method == "POST" {

			// bound the body before anything reads it
			// this will return a hard error but good safety stop
			r.Body = http.MaxBytesReader(w, r.Body, fileSize)
			if err := r.ParseMultipartForm(memLimit); err != nil {
				http.Error(w, "request too large to parse!", http.StatusBadRequest)
				return
			}

			// validate strings
			cleanTags, err := returnCleanSubmissionFields(selectedString, selectedTagString, selectedType)
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
			err, filetype := safeCheckContents(file)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			// actually save image
			err, filepath := safeSaveFile(file, filetype)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			submissionNode := Submission{
				ImagePath:        filepath,
				RelationshipType: selectedType,
				PotentialTags:    cleanTags,
			}

			// TODO: Create the node in the submission db table
			// check sql injection stuff
			fmt.Println(submissionNode)
			status = true

			templ.Handler(submission(selectedString, selectedTagString, selectedType, status)).ServeHTTP(w, r)
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
func returnCleanSubmissionFields(name string, tags string, searchtype string) ([]string, error) {

	// check tag size
	tagSplit := strings.Split(tags, ",")
	if len(tagSplit) > tagLimit {
		return []string{}, errors.New("Error: too many tags submitted")
	}

	// check searchtype is valid (code injection?)
	if slices.Contains(allowedSearches[:], searchtype) != true {
		return []string{}, errors.New("Error: no valid search type submitted")
	}

	// check name actually exists
	// enforce exact name
	// SQL inject?
	//if len(getnodeByName(name)) != 1 {
	//	return []string{}, errors.new("Error: could not find node with that name. Match must be exact!")
	//}

	// build and return tag array
	tagsArr := make([]string, len(tagSplit))
	for i, tag := range tagSplit {
		tagsArr[i] = strings.TrimSpace(tag)
	}

	return tagsArr, nil

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

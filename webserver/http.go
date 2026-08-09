package main

import (
	"database/sql"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
)

var lastUpload sync.Map

const rateLimit = 60 * time.Second
const fileSize = 2 * 1024 * 1024

var allowedTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

func startWebserver(db *sql.DB) {

	// serve the images from disk for now
	assetsDir := http.Dir(".")
	fs := http.FileServer(assetsDir)
	http.Handle("/assets/", fs)

	// serve html from templ
	http.Handle("/", templ.Handler(homepage(db)))

	http.HandleFunc("/subpage", makeSubpageHandler(db))

	http.HandleFunc("/submission", handleSubmission(db))

	http.Handle("/search", handleSearch(db))

	http.HandleFunc("/random", makeRandompageHandler(db))

	http.Handle("/about", templ.Handler(about()))

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
func handleSearch(db *sql.DB) http.HandlerFunc {
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

func handleSubmission(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			file, header, err := r.FormFile("image")
			if err != nil {
				http.Error(w, "No file uploaded", http.StatusBadRequest)
				return
			}
			defer file.Close()

			// check file size, type, and rate limit
			if err := validateUpload(r, header); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			// make dir for uploads
			err = os.Mkdir("./uploads/", 0750)
			if err != nil && !os.IsExist(err) {
				http.Error(w, "Failed to create uploads/", http.StatusInternalServerError)
				return
			}

			// Save to disk
			dst, err := os.Create("./uploads/" + header.Filename)
			if err != nil {
				http.Error(w, "Failed to save file", http.StatusInternalServerError)
				return
			}
			defer dst.Close()

			io.Copy(dst, file)
			templ.Handler(submission()).ServeHTTP(w, r)
			return
		}
		templ.Handler(submission()).ServeHTTP(w, r)
	}
}

func validateUpload(r *http.Request, header *multipart.FileHeader) error {

	// Check file size ( less than 2 MB )
	if header.Size > fileSize {
		return fmt.Errorf("file too large")
	}

	// Check file type
	// TODO: actually check contents and not just content-type?
	if !allowedTypes[header.Header.Get("Content-Type")] {
		return fmt.Errorf("Invalid file type")
	}

	// Check rate limit based on ip time
	ip := strings.Split(r.RemoteAddr, ":")[0]

	// remove old entries from the ip map if they have times 10x older
	// than the rate limit window (prevent unbounded growth)
	cleanupOldEntries(time.Now().Add(-rateLimit * 10))

	if last, ok := lastUpload.Load(ip); ok {
		if time.Since(last.(time.Time)) < rateLimit {
			return fmt.Errorf("rate limit exceeded: please wait %v", rateLimit)
		}
	}

	lastUpload.Store(ip, time.Now())
	return nil
}

func cleanupOldEntries(cutoffTime time.Time) {
	lastUpload.Range(func(key, value interface{}) bool {
		if value.(time.Time).Before(cutoffTime) {
			lastUpload.Delete(key)
		}
		return true
	})
}

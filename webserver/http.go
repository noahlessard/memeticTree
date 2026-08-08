package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
)

func startWebserver(db *sql.DB) {

	// serve the images from disk for now
	assetsDir := http.Dir(".")
	fs := http.FileServer(assetsDir)
	http.Handle("/assets/", fs)

	// serve html from templ
	http.Handle("/", templ.Handler(homepage(db)))

	http.HandleFunc("/subpage", makeSubpageHandler(db))

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

		// if form was submitted, search for results
		if r.Method == "POST" {
			searchQuery := r.FormValue("nodeinput")
			searchType = r.FormValue("searchType")
			if searchType == "name" {
				results = getnodeByName(db, searchQuery)
			} else {
				fmt.Println("got the following tags: " + searchQuery)
			}
		}

		// render the search component with or without results
		templ.Handler(search(results, searchType)).ServeHTTP(w, r)
	}
}

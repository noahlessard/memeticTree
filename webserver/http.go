package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"

	"github.com/a-h/templ"
)

func startWebserver(db *sql.DB) {

	// serve the images from disk for now
	assetsDir := http.Dir(".")
	fs := http.FileServer(assetsDir)
	http.Handle("/assets/", fs)

	// serve html from templ
	http.Handle("/", templ.Handler(homepage()))

	http.HandleFunc("/subpage", makeSubpageHandler(db))

	http.Handle("/search", handleSearch(db))

	http.HandleFunc("/random", makeRandompageHandler(db))

	http.Handle("/about", templ.Handler(about()))

	fmt.Println("Listening on :3000")
	http.ListenAndServe(":3000", nil)

}

func makeSubpageHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		node := getnode(db, 1) // this should be passed in the future or something
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

func handleSearch(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var results []Node

		// if form was submitted, search for results
		if r.Method == "POST" {
			searchQuery := r.FormValue("nodeinput")
			results = getnodeByName(db, searchQuery)
		}

		// render the search component with or without results
		templ.Handler(search(results)).ServeHTTP(w, r)
	}
}

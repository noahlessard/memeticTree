package main

import (
	"database/sql"
	"fmt"
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

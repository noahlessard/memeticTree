package main

import (
	"fmt"
	"net/http"

	"github.com/a-h/templ"
)

func main() {

	// serve the images from disk for now
	assetsDir := http.Dir(".")
	fs := http.FileServer(assetsDir)
	http.Handle("/assets/", fs)

	// serve html from templ
	component := homepage()
	http.Handle("/", templ.Handler(component))

	subpage := subpage()
	http.Handle("/subpage", templ.Handler(subpage))

	fmt.Println("Listening on :3000")
	http.ListenAndServe(":3000", nil)
}

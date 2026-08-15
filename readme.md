# Memetic Tree

![homepage](homepage.png)

The goal of this project is to provide very efficient image storage while also generating interesting anthropological statistics based off of submitted content. And, to have fun!

## Imported tech:
- gen2brain/avif for compression 
- mattn/go-sqlite3 for managing lookups, relations
- a-h/templ for managing front end parts
- podman for containerization (if needed?)

## How trees are organized:
- Each meme has ancestors and offspring.
- Memes can have multiple ancestors and multiple offspring.
- Individual offspring are referred to as `variants`.
- The original ancestor in a tree, with no other ancestors, is referred to as a `LUCA`. 
- `LUCA`s are dynamically updated as new data is added, since new ancestors will be discovered over time.

## Build:
- For Webserver:
    - Enter webserver directory 
    - Build templ files with: `go tool templ generate`
    - Run with `go run .`
- It is highly recommended to install the templ vscode extension if you are going to be working with the frontend, since templ files can be messy.
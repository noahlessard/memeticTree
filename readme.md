# Memetic Tree

The goal of this project is to provide very efficient image storage while also generate interesting anthropological statistics based off of submitted content. And, to have fun!

## Tech Stack:
- Store images as avif, super compressed, with jpg as a hot cache for frequent images
- gen2brain/jpegli + gen2brain/avif for compression 
- sqlite for managing lookups, relations (mattn/go-sqlite3)
- go to manage backend and statistics, user upload
- a-h/templ for managing front end parts
- podman for containerization (if needed?)
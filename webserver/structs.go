package main

import "time"

type Node struct {
	ID          string
	Name        string
	Description string
	Parents     []Node
	Children    []Node
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

package main

import "time"

type Node struct {
	ID          int
	Name        string
	Description string
	ImagePath   string
	Parents     []Node
	Children    []Node
	Tags        []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

package main

import "time"

type Node struct {
	ID          int
	Name        string
	Description string
	ImagePath   string
	Parents     []Node
	Children    []Node
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

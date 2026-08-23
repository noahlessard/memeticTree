package main

type Node struct {
	ID          int
	Name        string
	Description string
	ImagePath   string
	Parents     []Node
	Children    []Node
	Tags        []string
}

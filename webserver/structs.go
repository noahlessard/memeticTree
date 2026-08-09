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

type Submission struct {
	ImagePath        string
	RelationshipType string
	PotentialTags    []string
}

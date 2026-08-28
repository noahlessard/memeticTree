package main

import (
	"fmt"
	"html"
	"strings"
)

const gap = 2 // spacing reserved between sibling subtrees, in character columns

// a positioned node in the final layout: the column span its whole subtree
// reserves plus the single column its own label anchors on.
type layoutNode struct {
	id       int
	name     string
	width    int // subtree width in character columns
	center   int // column this node's own center sits on
	children []*layoutNode
}

// one printed line of the tree: either a row of aligned labels or a box-drawing connector.
type treeRow struct {
	name  bool
	cells []labelCell // for label rows: non-overlapping, ordered left to right
	line  string      // for connector rows: the full box-drawing line
}

// a single label of a row: the leading space before it and the text (and target) to draw.
type labelCell struct {
	spaces int // spaces to emit before this label (relative to previous)
	text   string
	id     int
}

// recursively mirror a Node's into a layoutNode, getting all children
// doesn't actual set any properties however
func createLayoutNode(n *Node) *layoutNode {
	ln := &layoutNode{id: n.ID, name: n.Name}
	for i := range n.Children {
		ln.children = append(ln.children, createLayoutNode(&n.Children[i]))
	}
	return ln
}

// returns the total width of a set of layout children including the gaps between siblings.
func getChildrenWidth(children []*layoutNode) int {
	total := 0
	for i, c := range children {
		if i > 0 {
			total += gap
		}
		total += c.width
	}
	return total
}

// bottom-up recursion setting each child's width
func computeWidths(n *layoutNode) {
	for _, c := range n.children {
		computeWidths(c)
	}
	// other base case: getting the width of children
	total := getChildrenWidth(n.children)
	n.width = max(total, len(n.name))
}

// top-down recurses through given nodes children, setting the center of each node in its span
func computeCenters(n *layoutNode, left int) {
	n.center = left + n.width/2

	cursor := n.center - getChildrenWidth(n.children)/2
	for i, c := range n.children {
		if i > 0 {
			cursor += gap
		}
		computeCenters(c, cursor)
		cursor += c.width
	}
}

// turns the root node in an array of treeRows, which each have the right sizing information.
func layoutTree(root Node) []treeRow {
	rn := createLayoutNode(&root)

	computeWidths(rn)
	computeCenters(rn, 0)

	// spread the tree into levels, top to bottom.
	var depth [][]*layoutNode
	depth = append(depth, []*layoutNode{rn})
	for {
		var next []*layoutNode
		for _, n := range depth[len(depth)-1] {
			next = append(next, n.children...)
		}
		if len(next) == 0 {
			break
		}
		depth = append(depth, next)
	}

	var rows []treeRow
	for d, level := range depth {
		lr := treeRow{name: true}
		lastEnd := 0
		for _, n := range level {
			col := n.center - len(n.name)/2
			if col < 0 {
				col = 0
			}
			if col < lastEnd {
				col = lastEnd
			}
			lr.cells = append(lr.cells, labelCell{spaces: col - lastEnd, text: n.name, id: n.id})
			lastEnd = col + len(n.name)
		}
		rows = append(rows, lr)

		if d < len(depth)-1 {
			conn := make([]rune, rn.width)
			for i := range conn {
				conn[i] = ' '
			}
			for _, n := range level {
				drawConnectors(conn, n)
			}
			rows = append(rows, treeRow{line: string(conn)})
		}
	}
	return rows
}

// drawConnectors draws one parent's portion of the connector row between a
// depth and its children. Sibling parents occupy disjoint column spans, so
// writing into the shared rune row is safe.
func drawConnectors(r []rune, p *layoutNode) {
	if len(p.children) == 0 {
		return
	}
	left := p.children[0].center
	right := p.children[len(p.children)-1].center
	lo := left
	if p.center < lo {
		lo = p.center
	}
	hi := right
	if p.center > hi {
		hi = p.center
	}
	for col := lo; col <= hi; col++ {
		up := col == p.center
		down := false
		for _, c := range p.children {
			if c.center == col {
				down = true
				break
			}
		}
		leftLine := col > lo
		rightLine := col < hi
		r[col] = connectorChar(up, down, leftLine, rightLine)
	}
}

func connectorChar(up, down, left, right bool) rune {
	switch {
	case up && down && left && right:
		return '┼'
	case up && down && left:
		return '┤'
	case up && down && right:
		return '├'
	case up && left && right:
		return '┴'
	case down && left && right:
		return '┬'
	case up && left:
		return '┘'
	case up && right:
		return '└'
	case down && left:
		return '┐'
	case down && right:
		return '┌'
	case up || down:
		return '│'
	default:
		return '─'
	}
}

// renderHTML builds the full pre content as one HTML string.
// Built in Go (not templ) so no template whitespace can break the fixed grid.
func renderHTML(root Node) string {
	var sb strings.Builder
	for _, row := range layoutTree(root) {
		sb.WriteString("\n")
		if row.name {
			for _, c := range row.cells {
				sb.WriteString(strings.Repeat(" ", c.spaces))
				sb.WriteString(`<a href="/subpage?id=`)
				sb.WriteString(fmt.Sprint(c.id))
				sb.WriteString(`">`)
				sb.WriteString(html.EscapeString(c.text))
				sb.WriteString(`</a>`)
			}
		} else {
			sb.WriteString(row.line)
		}
	}
	return sb.String()
}

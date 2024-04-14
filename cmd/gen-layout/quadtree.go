// adapted from https://github.com/jwhandley/graphyz/blob/main/quadtree.go
package main

import (
	"github.com/quartercastle/vector"
)

type QuadTree struct {
	Center    vector.Vector
	TotalMass float32
	Region    Rect
	Nodes     []*Node
	Children  [4]*QuadTree
}

type Rect struct {
	X, Y, Width, Height float32
}

func (r *Rect) Contains(pos vector.Vector) bool {
	contains := pos.X >= r.X && pos.X <= r.X+r.Width && pos.Y >= r.Y && pos.Y <= r.Y+r.Height
	return contains
}

func NewQuadTree(boundary Rect) *QuadTree {
	qt := new(QuadTree)
	qt.Region = boundary
	qt.Nodes = make([]*Node, 0, config.Capacity)
	qt.Children = [4]*QuadTree{nil, nil, nil, nil}
	qt.Center = vector.Vector{X: 0, Y: 0}
	qt.TotalMass = 0

	return qt
}

func (qt *QuadTree) Clear() {
	qt.Center = vector.VectorZero()
	qt.Nodes = nil
	for i := range qt.Children {
		qt.Children[i] = nil
	}
	qt.TotalMass = 0
}

func (qt *QuadTree) Insert(node *Node) bool {
	if !qt.Region.Contains(node.pos) {
		return false
	}

	if len(qt.Nodes) < config.Capacity {
		qt.Nodes = append(qt.Nodes, node)
		return true
	} else {
		if qt.Children[0] == nil {
			qt.Subdivide()
		}
		for _, child := range qt.Children {
			if child.Insert(node) {
				return true
			}
		}
	}
	return false
}

func (qt *QuadTree) Subdivide() {
	midX := qt.Region.X + qt.Region.Width/2
	midY := qt.Region.Y + qt.Region.Height/2

	halfWidth := (qt.Region.Width) / 2
	halfHeight := (qt.Region.Height) / 2

	qt.Children[0] = NewQuadTree(Rect{X: qt.Region.X, Y: qt.Region.Y, Width: halfWidth, Height: halfHeight}) // Top Left
	qt.Children[1] = NewQuadTree(Rect{X: midX, Y: qt.Region.Y, Width: halfWidth, Height: halfHeight})        // Top right
	qt.Children[2] = NewQuadTree(Rect{X: qt.Region.X, Y: midY, Width: halfWidth, Height: halfHeight})        // Bottom Left
	qt.Children[3] = NewQuadTree(Rect{X: midX, Y: midY, Width: halfWidth, Height: halfHeight})               // Bottom Right

	for _, node := range qt.Nodes {
		for _, child := range qt.Children {
			if child.Region.Contains(node.pos) {
				child.Insert(node)
				break
			}
		}
	}
}

func (qt *QuadTree) CalculateMasses() {
	if qt.Children[0] == nil {
		// Leaf
		for _, node := range qt.Nodes {
			qt.TotalMass += node.degree
			qt.Center.X += node.pos.X * node.degree
			qt.Center.Y += node.pos.Y * node.degree
		}
		qt.Center.X /= qt.TotalMass
		qt.Center.Y /= qt.TotalMass
	} else {
		// Process children
		for _, child := range qt.Children {
			child.CalculateMasses()
			qt.TotalMass += child.TotalMass
			qt.Center.X += child.Center.X * child.TotalMass
			qt.Center.Y += child.Center.Y * child.TotalMass
		}
		qt.Center.X /= qt.TotalMass
		qt.Center.Y /= qt.TotalMass
	}
}

func (qt *QuadTree) CalculateForce(node *Node, theta float32) vector.Vector {
	if qt.Children[0] == nil {
		totalForce := vector.VectorZero()
		for _, other := range qt.Nodes {
			force := calculateRepulsionForce(node, other)
			totalForce = vector.VectorAdd(totalForce, force)

		}
		return totalForce
	} else {
		d := vector.VectorDistance(node.pos, qt.Center)
		s := qt.Region.Width
		if (s / d) < theta {
			force := calculateRepulsionForce(node, qt)
			return force
		} else {
			totalForce := vector.VectorZero()
			for _, child := range qt.Children {
				if child != nil {
					totalForce = vector.VectorAdd(totalForce, child.CalculateForce(node, theta))
				}
			}
			return totalForce
		}
	}
}

func (qt *QuadTree) size() float32 {
	return qt.TotalMass
}

func (qt *QuadTree) position() vector.Vector {
	return qt.Center
}

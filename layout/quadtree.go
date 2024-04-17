// adapted from https://github.com/jwhandley/graphyz/blob/main/quadtree.go
package layout

import (
	"github.com/quartercastle/vector"
)

type QuadTreeConfig struct {
	CapacityOfEachBlock int
}

var QUADTREE_DEFAULT_CONFIG = QuadTreeConfig{CapacityOfEachBlock: 10}

type QuadTree struct {
	Center          vector.Vector
	TotalMass       float64
	Region          Rect
	Nodes           []*Node
	Children        [4]*QuadTree
	config          *QuadTreeConfig
	forceSimulation *ForceSimulation
}

type Rect struct {
	X, Y, Width, Height float64
}

func (r *Rect) Contains(pos vector.Vector) bool {
	contains := pos.X() >= r.X && pos.X() <= r.X+r.Width && pos.Y() >= r.Y && pos.Y() <= r.Y+r.Height
	return contains
}

func NewQuadTree(config *QuadTreeConfig, forceSimulation *ForceSimulation, boundary Rect) *QuadTree {
	qt := new(QuadTree)
	qt.config = config
	if config == nil {
		qt.config = &QUADTREE_DEFAULT_CONFIG
	}
	qt.Region = boundary
	qt.Nodes = make([]*Node, 0, qt.config.CapacityOfEachBlock)
	qt.Children = [4]*QuadTree{nil, nil, nil, nil}
	qt.Center = vector.Vector{0, 0}
	qt.TotalMass = 0
	return qt
}

func (qt *QuadTree) Clear() {
	qt.Center = vector.Vector{0, 0}
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

	if len(qt.Nodes) < qt.config.CapacityOfEachBlock {
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

	qt.Children[0] = NewQuadTree(qt.config, qt.forceSimulation, Rect{X: qt.Region.X, Y: qt.Region.Y, Width: halfWidth, Height: halfHeight}) // Top Left
	qt.Children[1] = NewQuadTree(qt.config, qt.forceSimulation, Rect{X: midX, Y: qt.Region.Y, Width: halfWidth, Height: halfHeight})        // Top right
	qt.Children[2] = NewQuadTree(qt.config, qt.forceSimulation, Rect{X: qt.Region.X, Y: midY, Width: halfWidth, Height: halfHeight})        // Bottom Left
	qt.Children[3] = NewQuadTree(qt.config, qt.forceSimulation, Rect{X: midX, Y: midY, Width: halfWidth, Height: halfHeight})               // Bottom Right

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
			qt.Center = qt.Center.Add(node.pos.Scale(node.degree))
		}
		qt.Center = qt.Center.Scale(1 / qt.TotalMass)
	} else {
		// Process children
		for _, child := range qt.Children {
			child.CalculateMasses()
			qt.TotalMass += child.TotalMass
			qt.Center = qt.Center.Add(child.Center.Scale(child.TotalMass))
		}
		qt.Center = qt.Center.Scale(1 / qt.TotalMass)
	}
}

func (qt *QuadTree) CalculateForce(node *Node, theta float64) vector.Vector {
	if qt.Children[0] == nil {
		totalForce := vector.Vector{0, 0}
		for _, other := range qt.Nodes {
			force := qt.forceSimulation.calculateRepulsionForce(node, other)
			totalForce = totalForce.Add(force)

		}
		return totalForce
	} else {
		d := node.pos.Sub(qt.Center).Magnitude()
		s := qt.Region.Width
		if (s / d) < theta {
			force := qt.forceSimulation.calculateRepulsionForce(node, qt)
			return force
		} else {
			totalForce := vector.Vector{0, 0}
			for _, child := range qt.Children {
				if child != nil {
					totalForce = totalForce.Add(child.CalculateForce(node, theta))
				}
			}
			return totalForce
		}
	}
}

func (qt *QuadTree) size() float64 {
	return qt.TotalMass
}

func (qt *QuadTree) position() vector.Vector {
	return qt.Center
}

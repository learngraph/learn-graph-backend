// adapted from https://github.com/jwhandley/graphyz/blob/main/quadtree.go
package layout

import (
	"sync"

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
	if qt.config.CapacityOfEachBlock == 0 {
		qt.config.CapacityOfEachBlock = 10
	}
	if config == nil {
		qt.config = &QUADTREE_DEFAULT_CONFIG
	}
	qt.Region = boundary
	qt.Nodes = make([]*Node, 0, qt.config.CapacityOfEachBlock)
	qt.Children = [4]*QuadTree{nil, nil, nil, nil}
	qt.Center = vector.Vector{0, 0}
	qt.TotalMass = 0
	qt.forceSimulation = forceSimulation
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
	return qt.insert(node, 0)
}

func (qt *QuadTree) insert(node *Node, depth int) bool {
	// FIXME(skep): if more than qt.forceSimulation.config.CapacityOfEachBlock
	// nodes are at the exact same location, then this is an inifite loop!
	// -> should wiggle those nodes a bit to divide them into different regions!
	if !qt.Region.Contains(node.Pos) {
		return false
	}

	if len(qt.Nodes) < qt.config.CapacityOfEachBlock {
		qt.Nodes = append(qt.Nodes, node)
		return true
	} else {
		if qt.Children[0] == nil {
			qt.subdivide(depth)
		}
		for _, child := range qt.Children {
			if child.insert(node, depth+1) {
				return true
			}
		}
	}
	return false
}

func (qt *QuadTree) subdivide(depth int) {
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
			if child.Region.Contains(node.Pos) {
				child.insert(node, depth+1)
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
			qt.Center = qt.Center.Add(node.Pos.Scale(node.degree))
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

// CalculateForce calculates the repulsion foce acting on a node.
// theta defines the accuracy of the simulation, see https://en.wikipedia.org/wiki/Barnes%E2%80%93Hut_simulation#Calculating_the_force_acting_on_a_body
func (qt *QuadTree) CalculateForce(node *Node, theta float64, parallelize int) vector.Vector {
	if qt.Children[0] == nil {
		totalForce := vector.Vector{0, 0}
		for _, other := range qt.Nodes {
			if node == other {
				continue
			}
			force := qt.forceSimulation.calculateRepulsionForce(node, other)
			vector.In(totalForce).Add(force)

		}
		return totalForce
	} else {
		d := node.Pos.Sub(qt.Center).Magnitude()
		s := qt.Region.Width
		if (s / d) < theta {
			force := qt.forceSimulation.calculateRepulsionForce(node, qt)
			return force
		} else {
			if false /*parallelize > 0*/ {
				totalForce := vector.Vector{0, 0}
				m := sync.Mutex{}
				wg := sync.WaitGroup{}
				for _, child := range qt.Children {
					if child == nil {
						continue
					}
					wg.Add(1)
					go func(child *QuadTree) {
						defer wg.Done()
						childForce := child.CalculateForce(node, theta, parallelize-1)
						m.Lock()
						defer m.Unlock()
						vector.In(totalForce).Add(childForce)
					}(child)
				}
				wg.Wait()
				return totalForce
			} else {
				totalForce := vector.Vector{0, 0}
				for _, child := range qt.Children {
					if child != nil {
						vector.In(totalForce).Add(child.CalculateForce(node, theta, 0))
					}
				}
				return totalForce
			}
		}
	}
}

// size() is used to compute repulsion force between QuadTrees
func (qt *QuadTree) size() float64 {
	return qt.TotalMass
}

func (qt *QuadTree) position() vector.Vector {
	return qt.Center
}

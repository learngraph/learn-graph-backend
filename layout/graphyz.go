// adapted from https://github.com/jwhandley/graphyz/blob/main/g.go
package layout

import (
	"math"
	"sync"

	"github.com/quartercastle/vector"
	"golang.org/x/exp/constraints"
)

type Body interface {
	size() float64
	position() vector.Vector
}

type Graph struct {
	Nodes           []*Node `json:"nodes"`
	Edges           []*Edge `json:"edges"`
	forceSimulation *ForceSimulation
}

type Node struct {
	Name     string `json:"name"`
	degree   float64
	isPinned bool
	radius   float64
	Pos      vector.Vector `json:"pos,omitempty"`
	vel      vector.Vector
	acc      vector.Vector
}

type Edge struct {
	Source int     `json:"source"`
	Target int     `json:"target"`
	Value  float64 `json:"value"`
}

// TODO(skep):  3D: point on sphere
func pointOnCircle(i, TotalPoints, Radius int, center vector.Vector) vector.Vector {
	return vector.Vector{
		math.Sin(float64(i) * 2.0 * math.Pi / float64(TotalPoints)),
		math.Cos(float64(i) * 2.0 * math.Pi / float64(TotalPoints)),
	}.Scale(float64(Radius)).Add(center)
}

func min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

func NewGraph(nodes []*Node, edges []*Edge, forceSimulation *ForceSimulation) *Graph {
	graph := Graph{
		Nodes:           nodes,
		Edges:           edges,
		forceSimulation: forceSimulation,
	}
	for _, edge := range graph.Edges {
		if edge.Value == 0.0 {
			edge.Value = 1.0
		}
		graph.Nodes[edge.Source].degree += edge.Value
		graph.Nodes[edge.Target].degree += edge.Value
	}
	for i, node := range graph.Nodes {
		if node.Pos.Magnitude() == 0 {
			if forceSimulation.conf.InitialLayout == InitialLayoutRandom {
				node.Pos = forceSimulation.conf.RandomVectorInside()
			} else if forceSimulation.conf.InitialLayout == InitialLayoutRandom {
				node.Pos = pointOnCircle(
					i, len(graph.Nodes),
					int(math.Floor(min(forceSimulation.conf.Rect.X, forceSimulation.conf.Rect.Y))),
					forceSimulation.conf.Rect.Center(),
				)
			}
		}
		if node.radius == 0 {
			node.radius = forceSimulation.conf.DefaultNodeRadius
		}
		if len(node.acc) == 0 {
			node.acc = vector.Vector{0, 0}
		}
		if len(node.vel) == 0 {
			node.vel = vector.Vector{0, 0}
		}
		if node.degree == 0.0 {
			node.degree = 1.0 // default degree != 0 is necessary for node<>node repulsion
		}
	}
	return &graph
}

// XXX: unused
func (g *Graph) resetPosition() {
	var initialRadius float64 = 10.0
	initialAngle := float64(math.Pi) * (3 - math.Sqrt(5))
	for i, node := range g.Nodes {
		radius := initialRadius * float64(math.Sqrt(0.5+float64(i)))
		angle := float64(i) * initialAngle

		node.Pos = vector.Vector{
			radius*float64(math.Cos(angle)) + float64(config.ScreenWidth)/2,
			radius*float64(math.Sin(angle)) + float64(config.ScreenHeight)/2,
		}
	}
}

func (g *Graph) ApplyForce(deltaTime float64, qt *QuadTree) {
	g.resetAcceleration()
	if g.forceSimulation.conf.Gravity {
		g.gravityToCenterForce()
	}

	g.attractionByEdgesForce()

	if config.BarnesHut {
		g.repulsionBarnesHut(qt)
	} else {
		g.repulsionNaive()
	}

	g.updatePositions(deltaTime)
}

func clamp(in, lo, hi float64) float64 {
	if math.IsNaN(in) {
		return in
	}
	if in > hi {
		return hi
	} else if in < lo {
		return lo
	}
	return in
}

func VectorClampValue(v vector.Vector, min, max float64) vector.Vector {
	return vector.Vector{
		clamp(v.X(), min, max),
		clamp(v.Y(), min, max),
	}
}
func VectorClampVector(v, min, max vector.Vector) vector.Vector {
	return vector.Vector{
		clamp(v.X(), min.X(), max.X()),
		clamp(v.Y(), min.Y(), max.Y()),
	}
}

func (g *Graph) updatePositions(deltaTime float64) {
	outOfBoundsFactor := g.forceSimulation.conf.ScreenMultiplierToClampPosition
	// TODO: should use g.forceSimulation.conf.Rect here instead of global config
	boundsMin := vector.Vector{
		-outOfBoundsFactor * float64(config.ScreenWidth), -outOfBoundsFactor * float64(config.ScreenHeight),
	}
	boundsMax := vector.Vector{
		outOfBoundsFactor * float64(config.ScreenWidth), outOfBoundsFactor * float64(config.ScreenHeight),
	}
	for _, node := range g.Nodes {
		if !node.isPinned {
			vector.In(node.vel).Add(node.acc)
			vector.In(node.vel).Scale(1 - config.VelocityDecay)
			node.vel = VectorClampValue(node.vel, -100, 100)
			vector.In(node.Pos).Add(node.vel.Scale(deltaTime))
			node.Pos = VectorClampVector(node.Pos, boundsMin, boundsMax)
		}
	}
}

func (g *Graph) resetAcceleration() {
	for _, node := range g.Nodes {
		node.acc = vector.Vector{0, 0}
	}
}

func (g *Graph) gravityToCenterForce() {
	center := g.forceSimulation.conf.Rect.Center()
	for _, node := range g.Nodes {
		delta := center.Sub(node.Pos)
		force := delta.Scale(g.forceSimulation.conf.GravityStrength * node.size() * g.forceSimulation.temperature)
		vector.In(node.acc).Add(force)
	}
}

func (g *Graph) attractionByEdgesForce() {
	for _, edge := range g.Edges {
		from := g.Nodes[edge.Source]
		to := g.Nodes[edge.Target]
		force := g.forceSimulation.calculateAttractionForce(from, to, edge.Value)
		vector.In(from.acc).Sub(force)
		vector.In(to.acc).Add(force)
	}
}

func (g *Graph) repulsionBarnesHut(qt *QuadTree) {
	qt.Clear()
	for _, node := range g.Nodes {
		qt.Insert(node)
	}
	qt.CalculateMasses()
	calculateForce := func(nodes []*Node) {
		for _, node := range nodes {
			force := vector.Vector{0, 0}
			tmp := vector.Vector{0, 0}
			qt.CalculateForce(&force, &tmp, node, config.Theta, g.forceSimulation.conf.Parallelization)
			vector.In(node.acc).Add(force)
		}
	}
	if g.forceSimulation.conf.Parallelization > 0 {
		total := len(g.Nodes)
		p := g.forceSimulation.conf.Parallelization
		wg := sync.WaitGroup{}
		wg.Add(p)
		for i := 0; i < p; i++ {
			go func(i int) {
				defer wg.Done()
				calculateForce(g.Nodes[i*total/p : (i+1)*total/p])
			}(i)
		}
		wg.Wait()
	} else {
		calculateForce(g.Nodes)
	}
}

func (g *Graph) repulsionNaive() {
	tmp := vector.Vector{0, 0}
	for i, node := range g.Nodes {
		for j, other := range g.Nodes {
			if i == j {
				continue
			}
			g.forceSimulation.calculateRepulsionForce(&node.acc, &tmp, node, other)
		}

	}
}

func (node *Node) size() float64 {
	return node.degree
}

func (node *Node) position() vector.Vector {
	return node.Pos
}

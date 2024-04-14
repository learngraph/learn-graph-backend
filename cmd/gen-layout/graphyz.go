// adapted from https://github.com/jwhandley/graphyz/blob/main/graph.go
package main

import (
	"math"

	"github.com/quartercastle/vector"
)

type Body interface {
	size() float64
	position() vector.Vector
}

type Graph struct {
	Nodes []*Node `json:"nodes"`
	Edges []*Edge `json:"links"`
}

var config = struct{
	ScreenWidth float64
	ScreenHeight float64
	VelocityDecay float64
	Gravity, BarnesHut bool
}{
	ScreenWidth: 100.0,
	ScreenHeight: 100.0,
	Gravity: false,
	BarnesHut: true,
	VelocityDecay: 0.1,
}

type Node struct {
	Name       string `json:"name"`
	Label      string `json:"label"`
	Group      int    `json:"group"`
	degree     float64
	isSelected bool
	radius     float64
	pos        vector.Vector
	vel        vector.Vector
	acc        vector.Vector
}

type Edge struct {
	Source int     `json:"source"`
	Target int     `json:"target"`
	Value  float64 `json:"value"`
}

func (graph *Graph) resetPosition() {
	var initialRadius float64 = 10.0
	initialAngle := float64(math.Pi) * (3 - math.Sqrt(5))
	for i, node := range graph.Nodes {
		radius := initialRadius * float64(math.Sqrt(0.5+float64(i)))
		angle := float64(i) * initialAngle

		node.pos = vector.Vector{
			radius*float64(math.Cos(angle)) + float64(config.ScreenWidth)/2,
			radius*float64(math.Sin(angle)) + float64(config.ScreenHeight)/2,
		}
	}
}

func (graph *Graph) ApplyForce(deltaTime float64, qt *QuadTree) {
	graph.resetAcceleration()
	if config.Gravity {
		graph.gravityForce()
	}

	graph.attractionForce()

	if config.BarnesHut {
		graph.repulsionBarnesHut(qt)
	} else {
		graph.repulsionNaive()
	}

	graph.updatePositions(deltaTime)
}

func VectorClampValue(v vector.Vector, min, max int) vector.Vector {
	return v // TODO: clamp x,y values to [min, max)
}
func VectorClampVector(v, min, max vector.Vector) vector.Vector {
	return v // TODO: clamp x,y values to [min, max)
}

func (graph *Graph) updatePositions(deltaTime float64) {
	for _, node := range graph.Nodes {
		if !node.isSelected {
			node.vel = node.vel.Add(node.acc)
			node.vel = node.vel.Scale(1-config.VelocityDecay)
			node.vel = VectorClampValue(node.vel, -100, 100)
			node.pos = node.pos.Add(node.vel.Scale(deltaTime))
			node.pos = VectorClampVector(node.pos, vector.Vector{
				-10*float64(config.ScreenWidth), -10*float64(config.ScreenHeight),
			}, vector.Vector{
				10*float64(config.ScreenWidth), 10*float64(config.ScreenHeight),
			})
		}
	}
}

func (graph *Graph) resetAcceleration() {
	for _, node := range graph.Nodes {
		node.acc = rl.Vector2Zero()
	}
}

func (graph *Graph) gravityForce() {
	center := vector.Vector{
		X: float64(config.ScreenWidth) / 2,
		Y: float64(config.ScreenHeight) / 2,
	}
	for _, node := range graph.Nodes {
		delta := rl.Vector2Subtract(center, node.pos)
		force := rl.Vector2Scale(delta, config.GravityStrength*node.size()*temperature)
		node.acc = rl.Vector2Add(node.acc, force)
	}
}

func (graph *Graph) attractionForce() {
	for _, edge := range graph.Edges {
		from := graph.Nodes[edge.Source]
		to := graph.Nodes[edge.Target]
		force := calculateAttractionForce(from, to, edge.Value)
		from.acc = rl.Vector2Subtract(from.acc, force)
		to.acc = rl.Vector2Add(to.acc, force)

	}
}

func (graph *Graph) repulsionBarnesHut(qt *QuadTree) {
	qt.Clear()

	for _, node := range graph.Nodes {
		qt.Insert(node)
	}
	qt.CalculateMasses()
	for _, node := range graph.Nodes {
		force := qt.CalculateForce(node, config.Theta)
		node.acc = rl.Vector2Add(node.acc, force)
	}
}

func (graph *Graph) repulsionNaive() {
	for i, node := range graph.Nodes {

		for j, other := range graph.Nodes {
			if i == j {
				continue
			}

			force := calculateRepulsionForce(node, other)
			node.acc = rl.Vector2Add(node.acc, force)
		}

	}
}

func (node *Node) size() float64 {
	return node.degree
}

func (node *Node) position() vector.Vector {
	return node.pos
}

func calculateRepulsionForce(b1 Body, b2 Body) vector.Vector {
	delta := rl.Vector2Subtract(b1.position(), b2.position())
	dist := rl.Vector2LengthSqr(delta)
	if dist*dist < b1.size()*b2.size() {
		dist = b1.size() * b2.size()
	}
	scale := b1.size() * b2.size() * temperature
	force := rl.Vector2Scale(rl.Vector2Normalize(delta), 10*scale/dist)
	return force
}

func calculateAttractionForce(from *Node, to *Node, weight float64) vector.Vector {
	delta := rl.Vector2Subtract(from.pos, to.pos)
	dist := rl.Vector2Length(delta)

	if dist < EPSILON {
		dist = EPSILON
	}
	s := float64(math.Min(float64(from.radius), float64(to.radius)))
	var l float64 = from.radius + to.radius
	return rl.Vector2Scale(rl.Vector2Normalize(delta), (dist-l)/s*weight*temperature)
}

package layout

import (
	"testing"

	"github.com/quartercastle/vector"
	"github.com/stretchr/testify/assert"
)

func TestNewGraph(t *testing.T) {
	rect := Rect{X: 0, Y: 0, Width: 10, Height: 10}
	g := NewGraph(
		[]*Node{{}, {}},
		[]*Edge{{Source: 0, Target: 1, Value: 5}},
		NewForceSimulation(ForceSimulationConfig{Rect: rect}),
	)
	assert := assert.New(t)
	assert.Equal(g.Nodes[0].degree, 5.0)
	assert.Equal(g.Nodes[1].degree, 5.0)
	assert.NotZero(g.Nodes[1].pos.Sub(g.Nodes[0].pos).Magnitude(), "nodes should be initialized randomly")
	assert.NotZero(g.Nodes[0].radius)
	assert.NotZero(g.Nodes[1].radius)
}

func TestGraph_updatePositions(t *testing.T) {
	rect := Rect{X: 0, Y: 0, Width: 10, Height: 10}
	fs := NewForceSimulation(ForceSimulationConfig{Rect: rect})
	fs.temperature = 1.0
	g := NewGraph(
		[]*Node{{pos: vector.Vector{1, 1}}, {pos: vector.Vector{2, 2}}},
		[]*Edge{{Source: 0, Target: 1, Value: 5}},
		fs,
	)
	assert := assert.New(t)
	g.repulsionNaive()
	g.updatePositions(0.1)
	assert.Lessf(g.Nodes[0].pos.X(), 1.0, "should move nodes away from each other")
	assert.Greaterf(g.Nodes[1].pos.X(), 2.0, "should move nodes away from each other")
}

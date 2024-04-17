package layout

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGraph(t *testing.T) {
	assert := assert.New(t)
	rect := Rect{X: 0, Y: 0, Width: 10, Height: 10}
	g := NewGraph(
		[]*Node{{}, {}},
		[]*Edge{{Source: 0, Target: 1, Value: 5}},
		NewForceSimulation(ForceSimulationConfig{Rect: rect}),
	)
	assert.Equal(g.Nodes[0].degree, 5.0)
	assert.Equal(g.Nodes[1].degree, 5.0)
	assert.NotZero(g.Nodes[1].pos.Sub(g.Nodes[0].pos).Magnitude(), "nodes should be initialized randomly")
	assert.NotZero(g.Nodes[0].radius)
	assert.NotZero(g.Nodes[1].radius)
}

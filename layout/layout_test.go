package layout

import (
	"testing"

	"github.com/quartercastle/vector"
	"github.com/stretchr/testify/assert"
)

func DISTestForceSimulation(t *testing.T) {
	fs := NewForceSimulation(DefaultForceSimulationConfig)
	nodes := fs.ComputeLayout(
		[]*Node{{Name: "A", pos: vector.Vector{1, 1}}, {Name: "B", pos: vector.Vector{2, 2}}},
		[]*Edge{{Source: 0, Target: 1}},
	)
	exp := []*Node{
		{Name: "A", degree: 1, pos: vector.Vector{1, 1}},
		{Name: "B", degree: 1, pos: vector.Vector{2, 2}},
	}
	assert.Equal(t, exp, nodes)
}

package layout

import (
	"math/rand"
	"testing"

	"github.com/quartercastle/vector"
	"github.com/stretchr/testify/assert"
)

func TestForceSimulation(t *testing.T) {
	for _, test := range []struct {
		Name       string
		Config     ForceSimulationConfig
		Nodes      []*Node
		Edges      []*Edge
		Assertions func(t *testing.T, nodes []*Node)
	}{
		{
			Name:  "push 2 nodes apart (even though connected!)",
			Nodes: []*Node{{Name: "A", pos: vector.Vector{9, 9}}, {Name: "B", pos: vector.Vector{10, 10}}},
			Edges: []*Edge{{Source: 0, Target: 1}},
			Assertions: func(t *testing.T, nodes []*Node) {
				assert := assert.New(t)
				assert.Less(nodes[0].pos.X(), 8.0)
				assert.Less(nodes[0].pos.Y(), 8.0)
				assert.Greater(nodes[1].pos.X(), 10.0)
				assert.Greater(nodes[1].pos.Y(), 10.0)
			},
			Config: ForceSimulationConfig{RandomFloat: func() float64 { return 1.0 }},
		},
		{
			Name:  "pull 2 nodes together by edge",
			Nodes: []*Node{{Name: "A", pos: vector.Vector{1, 1}}, {Name: "B", pos: vector.Vector{200, 200}}},
			Edges: []*Edge{{Source: 0, Target: 1}},
			Assertions: func(t *testing.T, nodes []*Node) {
				assert := assert.New(t)
				// expect, that the equilibrium settles somewhere around P=(100,100)
				// n1, n2 ∈ (90, 100)
				assert.Greater(nodes[0].pos.X(), 90.0)
				assert.Greater(nodes[0].pos.Y(), 90.0)
				assert.Less(nodes[0].pos.X(), 110.0)
				assert.Less(nodes[0].pos.Y(), 110.0)
				assert.Greater(nodes[1].pos.X(), 90.0)
				assert.Greater(nodes[1].pos.Y(), 90.0)
				assert.Less(nodes[1].pos.X(), 110.0)
				assert.Less(nodes[1].pos.Y(), 110.0)
			},
			Config: ForceSimulationConfig{RandomFloat: func() float64 { return 1.0 }},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			fs := NewForceSimulation(test.Config)
			nodes, _ := fs.ComputeLayout(test.Nodes, test.Edges)
			test.Assertions(t, nodes)
		})
	}
}

func BenchmarkForceSimulation(b *testing.B) {
	for n := 10; n < b.N; n += 10 {
		fs := NewForceSimulation(DefaultForceSimulationConfig)
		nodes := []*Node{}
		edges := []*Edge{}
		for i := 0; i < n; i++ {
			nodes = append(nodes, &Node{})
		}
		for i := 0; i < n; i++ {
			edge := Edge{Source: rand.Intn(n), Target: rand.Intn(n)}
			if edge.Source == edge.Target {
				if edge.Target == n {
					edge.Target = edge.Source - 1
				} else {
					edge.Target = edge.Source + 1
				}
			}
			edges = append(edges, &edge)
		}
		b.StartTimer()
		fs.ComputeLayout(nodes, edges)
		b.StopTimer()
	}
}

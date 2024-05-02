package layout

import (
	"math"
	"testing"

	"github.com/quartercastle/vector"
	"github.com/stretchr/testify/assert"
)

func TestQUandTree_New(t *testing.T) {
	fs := NewForceSimulation(ForceSimulationConfig{})
	qt := NewQuadTree(&QuadTreeConfig{CapacityOfEachBlock: 2}, fs, Rect{X: 0, Y: 0, Width: 10.0, Height: 10.0})
	p11, p22 := &Node{Pos: vector.Vector{1.0, 1.0}}, &Node{Pos: vector.Vector{2.0, 2.0}}
	assert := assert.New(t)
	assert.True(qt.Insert(p11))
	assert.True(qt.Insert(p22))
	assert.Equal([]*Node{p11, p22}, qt.Nodes)
	for i := 0; i < 4; i++ {
		assert.Nil(qt.Children[i], "children should not exist, below CapacityOfEachBlock")
	}
	p33 := &Node{Pos: vector.Vector{3.0, 3.0}}
	assert.True(qt.Insert(p33))
	assert.Equal([]*Node{p11, p22}, qt.Nodes)
	assert.NotNil(qt.Children[0])
	assert.Equal([]*Node{p11, p22}, qt.Children[0].Nodes)
	assert.Equal([]*Node{}, qt.Children[1].Nodes)
	assert.Equal([]*Node{}, qt.Children[2].Nodes)
	assert.Equal([]*Node{}, qt.Children[3].Nodes)
}

func TestQUandTree_CalculateMasses(t *testing.T) {
	rect := Rect{X: 0.0, Y: 0.0, Width: 10.0, Height: 10.0}
	fs := NewForceSimulation(ForceSimulationConfig{Rect: rect})
	qt := NewQuadTree(&QuadTreeConfig{CapacityOfEachBlock: 2}, fs, rect)
	graph := NewGraph(
		[]*Node{
			{Name: "A", Pos: vector.Vector{2.5, 2.5}},
			{Name: "B", Pos: vector.Vector{7.5, 2.5}},
			{Name: "C", Pos: vector.Vector{2.5, 7.5}},
		},
		[]*Edge{{Source: 0, Target: 1}, {Source: 1, Target: 2}},
		fs,
	)
	for _, n := range graph.Nodes {
		qt.Insert(n)
	}
	qt.CalculateMasses()
	assert := assert.New(t)
	assert.True(math.IsNaN(qt.Center.X()), "top level node has no meaningful center")
	assert.True(math.IsNaN(qt.Center.Y()), "top level node has no meaningful center")
	assert.Equal(vector.Vector{2.5, 2.5}, qt.Children[0].Center)
	assert.Equal(vector.Vector{7.5, 2.5}, qt.Children[1].Center)
	assert.Equal(vector.Vector{2.5, 7.5}, qt.Children[2].Center)
	assert.True(math.IsNaN(qt.Children[3].Center.X()), "all 3 nodes already in first 3 buckets")
	assert.True(math.IsNaN(qt.Children[3].Center.Y()), "all 3 nodes already in first 3 buckets")
}

func TestQUandTree_CalculateForce(t *testing.T) {
	conf := ForceSimulationConfig{Rect: Rect{X: 0.0, Y: 0.0, Width: 10.0, Height: 10.0}}
	fs := NewForceSimulation(conf)
	qt := NewQuadTree(&QuadTreeConfig{CapacityOfEachBlock: 2}, fs, conf.Rect)
	graph := NewGraph(
		[]*Node{
			{Name: "A", Pos: vector.Vector{2.5, 2.5}},
			{Name: "B", Pos: vector.Vector{7.5, 2.5}},
			{Name: "C", Pos: vector.Vector{2.5, 7.5}},
		},
		[]*Edge{{Source: 0, Target: 1}, {Source: 1, Target: 2}},
		fs,
	)
	for _, n := range graph.Nodes {
		qt.Insert(n)
	}
	force := vector.Vector{0, 0}
	tmp := vector.Vector{0, 0}
	qt.CalculateForce(&force, &tmp, graph.Nodes[0], 0.1, 0)
	assert := assert.New(t)
	assert.Equal(vector.Vector{-4.0, -2.0}, force)
	forceParallel := vector.Vector{0, 0}
	qt.CalculateForce(&forceParallel, &tmp, graph.Nodes[0], 0.1, 1)
	assert.Equal(vector.Vector{-4.0, -2.0}, forceParallel)
}

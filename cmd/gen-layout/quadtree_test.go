package main

import (
	"math"
	"testing"

	"github.com/quartercastle/vector"
	"github.com/stretchr/testify/assert"
)

func TestQUandTree_New(t *testing.T) {
	assert := assert.New(t)
	qt := NewQuadTree(&QuadTreeConfig{CapacityOfEachBlock: 2}, Rect{X: 0, Y: 0, Width: 10.0, Height: 10.0})
	p11, p22 := &Node{pos: vector.Vector{1.0, 1.0}}, &Node{pos: vector.Vector{2.0, 2.0}}
	assert.True(qt.Insert(p11))
	assert.True(qt.Insert(p22))
	assert.Equal([]*Node{p11, p22}, qt.Nodes)
	for i := 0; i < 4; i++ {
		assert.Nil(qt.Children[i], "children should not exist, below CapacityOfEachBlock")
	}
	p33 := &Node{pos: vector.Vector{3.0, 3.0}}
	assert.True(qt.Insert(p33))
	assert.Equal([]*Node{p11, p22}, qt.Nodes)
	assert.NotNil(qt.Children[0])
	assert.Equal([]*Node{p11, p22}, qt.Children[0].Nodes)
	assert.Equal([]*Node{}, qt.Children[1].Nodes)
	assert.Equal([]*Node{}, qt.Children[2].Nodes)
	assert.Equal([]*Node{}, qt.Children[3].Nodes)
}

func TestQUandTree_CalculateMasses(t *testing.T) {
	qt := NewQuadTree(&QuadTreeConfig{CapacityOfEachBlock: 2}, Rect{X: 0, Y: 0, Width: 10.0, Height: 10.0})
	rect := Rect{X: 0, Y: 0, Width: 10, Height: 10}
	graph := NewGraph(
		[]*Node{
			{Name: "A", pos: vector.Vector{2.5, 2.5}},
			{Name: "B", pos: vector.Vector{7.5, 2.5}},
			{Name: "C", pos: vector.Vector{2.5, 7.5}}},
		[]*Edge{{Source: 0, Target: 1}, {Source: 1, Target: 2}},
		rect,
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

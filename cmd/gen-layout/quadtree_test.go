package main

import (
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
	qt.CalculateMasses()
	// TODO
	//assert := assert.New(t)
	//assert.Equal(vector.Vector{0.0, 0.0}, qt.Center)
}

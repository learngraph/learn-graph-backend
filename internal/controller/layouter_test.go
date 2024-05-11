package controller

import (
	"context"
	"testing"

	"github.com/quartercastle/vector"
	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
	"github.com/suxatcode/learn-graph-poc-backend/layout"
)

func TestForceSimulationLayouter_GetNodePositions_simple(t *testing.T) {
	l := NewForceSimulationLayouter()
	l.lnodes = []*layout.Node{
		{Name: "1", Pos: vector.Vector{1, 2, 3}}, {Name: "2", Pos: vector.Vector{3, 4, 5}},
	}
	l.layoutToModelLUT = map[int]string{
		0: "1",
		1: "2",
	}
	g := &model.Graph{
		Nodes: []*model.Node{{ID: "1"}, {ID: "2"}},
	}
	l.GetNodePositions(context.Background(), g)
	assert := assert.New(t)
	assert.Equal([]*model.Node{
		{ID: "1", Position: &model.Vector{X: 1, Y: 2, Z: 3}}, {ID: "2", Position: &model.Vector{X: 3, Y: 4, Z: 5}},
	}, g.Nodes)
}
func TestForceSimulationLayouter_GetNodePositions_notOrdered(t *testing.T) {
	l := NewForceSimulationLayouter()
	l.lnodes = []*layout.Node{
		{Name: "2", Pos: vector.Vector{3, 4, 5}}, {Name: "1", Pos: vector.Vector{1, 2, 3}},
	}
	l.layoutToModelLUT = map[int]string{
		0: "2",
		1: "1",
	}
	g := &model.Graph{
		Nodes: []*model.Node{{ID: "2"}, {ID: "1"}},
	}
	l.GetNodePositions(context.Background(), g)
	assert := assert.New(t)
	assert.Equal([]*model.Node{
		{ID: "2", Position: &model.Vector{X: 3, Y: 4, Z: 5}}, {ID: "1", Position: &model.Vector{X: 1, Y: 2, Z: 3}},
	}, g.Nodes)
}

// snapshot test that force simulation is executed
func TestForceSimulationLayouter_Reload(t *testing.T) {
	l := NewForceSimulationLayouter()
	g := &model.Graph{
		Nodes: []*model.Node{{Description: "2"}, {Description: "1"}},
		Edges: []*model.Edge{{From: "1", To: "2", Weight: 5.0}},
	}
	l.Reload(context.Background(), g)
	assert := assert.New(t)
	for i, node := range []*layout.Node{
		{Name: "2", Pos: vector.Vector{994.3818795135339, 498.66956566006263}},
		{Name: "1", Pos: vector.Vector{1011.2362409729319, 502.66086867987485}},
	} {
		assert.Equal(node.Name, l.lnodes[i].Name)
		assert.Equal(node.Pos, l.lnodes[i].Pos)
	}
}

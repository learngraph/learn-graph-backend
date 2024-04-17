package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewGraph(t *testing.T) {
	assert := assert.New(t)
	g := NewGraph([]*Node{{}, {}}, []*Edge{{Source: 0, Target: 1, Value: 5}})
	assert.Equal(g.Nodes[0].degree, 5.0)
	assert.Equal(g.Nodes[1].degree, 5.0)
}

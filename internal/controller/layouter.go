package controller

import (
	"context"
	"runtime"

	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
	"github.com/suxatcode/learn-graph-poc-backend/layout"
)

//go:generate mockgen -destination layout_mock.go -package controller . Layouter
type Layouter interface {
	// GetNodePositions assigns node possitions from a past graph embedding
	// run. This is a quick call, that does no perform graph embedding.
	GetNodePositions(context.Context, *model.Graph)
	// Reload re-runs graph embedding. This is a synchronous call and will
	// take some time.
	Reload(context.Context, *model.Graph)
}

// implements Layouter
type ForceSimulationLayouter struct {
	completeSimulation *layout.ForceSimulation
	lnodes             []*layout.Node
	ledges             []*layout.Edge
	modelToLayoutLUT   map[string]int
	layoutToModelLUT   map[int]string
	//quickSimulation *layout.ForceSimulation
}

func XXXREMOVEAddPreComputedNodePositions(ctx context.Context, g *model.Graph) {
	max_y := 1000.0
	fs := layout.NewForceSimulation(layout.ForceSimulationConfig{
		Rect: layout.Rect{X: 0.0, Y: 0.0, Width: max_y * 2, Height: max_y}, ScreenMultiplierToClampPosition: 100,
		FrameTime:              1.0,   // default: 0.016
		MinDistanceBeweenNodes: 100.0, // default: 1e-2
		AlphaInit:              1.0,
		AlphaDecay:             0.005,
		AlphaTarget:            0.10,
		RepulsionMultiplier:    10.0, // default: 10.0
		Parallelization:        runtime.NumCPU() * 2,
		Gravity:                true,
		GravityStrength:        0.1,
	})
	lnodes, ledges := []*layout.Node{}, []*layout.Edge{}
	nodeIDLookup := make(map[string]int)
	for index, node := range g.Nodes {
		lnodes = append(lnodes, &layout.Node{Name: node.Description})
		nodeIDLookup[node.ID] = index
	}
	for _, edge := range g.Edges {
		ledges = append(ledges, &layout.Edge{Source: nodeIDLookup[edge.From], Target: nodeIDLookup[edge.To]})
	}
	_, stats := fs.ComputeLayout(ctx, lnodes, ledges)
	for i := range g.Nodes {
		g.Nodes[i].Position = &model.Vector{X: lnodes[i].Pos.X(), Y: lnodes[i].Pos.Y()}
	}
	println(&stats)
}

func NewForceSimulationLayouter() *ForceSimulationLayouter {
	max_y := 1000.0
	rect := layout.Rect{X: 0.0, Y: 0.0, Width: max_y * 2, Height: max_y}
	return &ForceSimulationLayouter{
		completeSimulation: layout.NewForceSimulation(layout.ForceSimulationConfig{
			Rect:                            rect,
			ScreenMultiplierToClampPosition: 100,
			FrameTime:                       1.0,   // default: 0.016
			MinDistanceBeweenNodes:          100.0, // default: 1e-2
			AlphaInit:                       1.0,
			AlphaDecay:                      0.005,
			AlphaTarget:                     0.10,
			RepulsionMultiplier:             10.0,                 // default: 10.0
			Parallelization:                 runtime.NumCPU() * 2, // x2, since distribution of nodes is not balanced
			Gravity:                         true,
			GravityStrength:                 0.1,
		}),
		//quickSimulation: layout.NewForceSimulation(layout.ForceSimulationConfig{
		//	Rect: rect,
		//	ScreenMultiplierToClampPosition: 100,
		//	FrameTime:              1.0,   // default: 0.016
		//	MinDistanceBeweenNodes: 100.0, // default: 1e-2
		//	AlphaInit:              1.0,
		//	AlphaDecay:             0.005,
		//	AlphaTarget:            0.10,
		//	RepulsionMultiplier:    10.0,                 // default: 10.0
		//	Parallelization:        runtime.NumCPU() * 2, // x2, since distribution of nodes is not balanced
		//	Gravity:                true,
		//	GravityStrength:        0.1,
		//}),
	}
}
func (l *ForceSimulationLayouter) GetNodePositions(ctx context.Context, g *model.Graph) {
	modelNodeIndexByID := make(map[string]int, len(g.Nodes))
	for i, node := range g.Nodes {
		modelNodeIndexByID[node.ID] = i
	}
	for i, node := range l.lnodes {
		id := l.layoutToModelLUT[i]
		idx := modelNodeIndexByID[id]
		g.Nodes[idx].Position = &model.Vector{
			X: node.Pos.X(),
			Y: node.Pos.Y(),
			Z: node.Pos.Z(),
		}
	}
}
func (l *ForceSimulationLayouter) Reload(ctx context.Context, g *model.Graph) {
}

package controller

import (
	"context"
	"runtime"

	"github.com/rs/zerolog/log"
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
	Reload(context.Context, *model.Graph) layout.Stats
}

// implements Layouter
type ForceSimulationLayouter struct {
	completeSimulation      *layout.ForceSimulation
	lnodes                  []*layout.Node
	ledges                  []*layout.Edge
	modelToLayoutNodeLookup map[string]int
	modelToLayoutEdgeLookup map[string]int
	quickSimulation         *layout.ForceSimulation
	// XXX: dirty hack, should clean up a bit
	shouldReRunAfterQuick bool
}

func NewForceSimulationLayouter() *ForceSimulationLayouter {
	max_y := 1000.0
	rect := layout.Rect{X: -max_y, Y: -max_y / 2, Width: max_y * 2, Height: max_y}
	config := layout.ForceSimulationConfig{
		InitialLayout:                   layout.InitialLayoutCircle,
		Rect:                            rect,
		ScreenMultiplierToClampPosition: 100,
		FrameTime:                       1.0,
		MinDistanceBeweenNodes:          100.0,
		AlphaInit:                       1.0,
		AlphaDecay:                      0.005,
		AlphaTarget:                     0.10,
		RepulsionMultiplier:             10.0,                 // default: 10.0
		Parallelization:                 runtime.NumCPU() * 2, // x2, since distribution of nodes is not balanced
		Gravity:                         true,
		GravityStrength:                 0.1,
	}
	configQuickSim := config
	configQuickSim.AlphaInit = 10.0
	configQuickSim.AlphaDecay = 0.2
	configQuickSim.AlphaTarget = 1
	configQuickSim.InitialLayout = layout.InitialLayoutRandom // TODO(skep): create InitialLayoutCloseToFirstEdgeFound
	return &ForceSimulationLayouter{
		completeSimulation: layout.NewForceSimulation(config),
		quickSimulation:    layout.NewForceSimulation(configQuickSim),
	}
}

// FIXME(skep): concurrency hell: not save since it uses modelToLayoutEdgeLookup
func (l *ForceSimulationLayouter) getNewNodesAndEdges(g *model.Graph) ([]*model.Node, []*model.Edge) {
	if l.modelToLayoutNodeLookup == nil || l.modelToLayoutEdgeLookup == nil {
		return []*model.Node{}, []*model.Edge{}
	}
	missingNodes := []*model.Node{}
	for _, node := range g.Nodes {
		if _, exists := l.modelToLayoutNodeLookup[node.ID]; !exists {
			missingNodes = append(missingNodes, node)
		}
	}
	missingEdges := []*model.Edge{}
	for _, edge := range g.Edges {
		if _, exists := l.modelToLayoutEdgeLookup[edge.ID]; !exists {
			missingEdges = append(missingEdges, edge)
		}
	}
	return missingNodes, missingEdges
}

func (l *ForceSimulationLayouter) GetNodePositions(ctx context.Context, g *model.Graph) {
	modelNodeIndexByID := make(map[string]int, len(g.Nodes))
	modelEdgeIndexByID := make(map[string]int, len(g.Edges))
	for i, node := range g.Nodes {
		modelNodeIndexByID[node.ID] = i
	}
	for i, edge := range g.Edges {
		modelEdgeIndexByID[edge.ID] = i
	}
	// FIXME(skep): lock node positions during quick sim?! See Reload() below.
	missingNodes, missingEdges := l.getNewNodesAndEdges(g)
	for _, node := range l.lnodes {
		node.IsPinned = true
	}
	if len(missingNodes) > 0 || len(missingEdges) > 0 {
		newNodes, _ := l.appendNodesAndEdges(missingNodes, missingEdges)
		l.shouldReRunAfterQuick = true
		l.quickSimulation.InitializeNodes(ctx, newNodes)                     // initialize only new nodes
		_, stats := l.quickSimulation.ComputeLayout(ctx, l.lnodes, l.ledges) // run quickSimulation with all nodes & edges
		l.updateLayoutWith(g, l.lnodes)
		log.Info().Msgf(
			"*quick* graph layout computaton finished: stats{iterations: %d, time: %d ms}",
			stats.Iterations,
			stats.TotalTime.Milliseconds(),
		)
	}
	for i, node := range g.Nodes {
		idx := l.modelToLayoutNodeLookup[node.ID]
		node := l.lnodes[idx]
		g.Nodes[i].Position = &model.Vector{
			X: node.Pos.X(),
			Y: node.Pos.Y(),
			Z: node.Pos.Z(),
		}
	}
}

// TODO(skep): detect deleted edges/nodes in `g`
func (l *ForceSimulationLayouter) shouldRun(g *model.Graph) bool {
	if l.modelToLayoutNodeLookup == nil || l.modelToLayoutEdgeLookup == nil {
		return true // initial run
	}
	if l.shouldReRunAfterQuick {
		l.shouldReRunAfterQuick = false
		return true
	}
	missingNodes, missingEdges := l.getNewNodesAndEdges(g)
	if len(missingNodes) == 0 && len(missingEdges) == 0 {
		return false
	}
	return true
}

// FIXME(skep): The position of the nodes is updated live during the
// simulation, so it is possible for a user to make a request when the
// simulation starts and get a very bad visual result.
func (l *ForceSimulationLayouter) Reload(ctx context.Context, g *model.Graph) layout.Stats {
	if !l.shouldRun(g) {
		return layout.Stats{}
	}
	l.lnodes, l.ledges = []*layout.Node{}, []*layout.Edge{}
	l.modelToLayoutNodeLookup = make(map[string]int, len(g.Nodes))
	l.modelToLayoutEdgeLookup = make(map[string]int, len(g.Edges))
	l.appendNodesAndEdges(g.Nodes, g.Edges)
	l.completeSimulation.InitializeNodes(ctx, l.lnodes)
	_, stats := l.completeSimulation.ComputeLayout(ctx, l.lnodes, l.ledges)
	l.updateLayoutWith(g, l.lnodes)
	return stats
}

func (l *ForceSimulationLayouter) updateLayoutWith(g *model.Graph, nodes []*layout.Node) {
	for i := range g.Nodes {
		g.Nodes[i].Position = &model.Vector{X: nodes[i].Pos.X(), Y: nodes[i].Pos.Y()}
	}
}

// returns newly added nodes and edges as layout.{Node/Edge} type
func (l *ForceSimulationLayouter) appendNodesAndEdges(nodes []*model.Node, edges []*model.Edge) ([]*layout.Node, []*layout.Edge) {
	newNodes := []*layout.Node{}
	for index, node := range nodes {
		newNodes = append(newNodes, &layout.Node{Name: node.Description})
		l.modelToLayoutNodeLookup[node.ID] = index + len(l.lnodes)
	}
	newEdges := []*layout.Edge{}
	for index, edge := range edges {
		newEdges = append(newEdges, &layout.Edge{Source: l.modelToLayoutNodeLookup[edge.From], Target: l.modelToLayoutNodeLookup[edge.To]})
		l.modelToLayoutEdgeLookup[edge.ID] = index + len(l.ledges)
	}
	l.lnodes = append(l.lnodes, newNodes...)
	l.ledges = append(l.ledges, newEdges...)
	return newNodes, newEdges
}

package controller

import (
	"context"
	"runtime"

	"github.com/rs/zerolog/log"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
	"github.com/suxatcode/learn-graph-poc-backend/layout"
)

// Layouter is an interface for integration of a periodic graph embedding
// computation into the graph-query infrastructure of the learngraph.
//
//go:generate mockgen -destination layout_mock.go -package controller . Layouter
type Layouter interface {
	// GetNodePositions assigns node possitions from a past graph embedding
	// run. This is a quick call, that does no perform graph embedding.
	GetNodePositions(context.Context, *model.Graph)
	// Reload re-runs graph embedding. This is a synchronous call and will
	// take some time.
	Reload(context.Context, *model.Graph) layout.Stats
}

// NewLayouter returns an implementation of the Layouter interface.
func NewLayouter() Layouter {
	return NewForceSimulationLayouter()
}

// implements Layouter
// Idea:
//   - run a completeSimulation when layout changes to the graph happen, and
//   - run a quickSimulation on every request IFF the current layout is missing
//     some node/edge.
type ForceSimulationLayouter struct {
	completeSimulation *layout.ForceSimulation
	simulationState    *simulationState
	quickSimulation    *layout.ForceSimulation
	// GetNodePositions will always wait for this channel and is closed after
	// the first Reload()
	waitForInitialLayout chan bool
	initialLayoutDone    bool
}

type simulationState struct {
	lnodes                  []*layout.Node
	ledges                  []*layout.Edge
	modelToLayoutNodeLookup map[string]int
	modelToLayoutEdgeLookup map[string]int
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
		completeSimulation:   layout.NewForceSimulation(config),
		simulationState:      &simulationState{},
		quickSimulation:      layout.NewForceSimulation(configQuickSim),
		waitForInitialLayout: make(chan bool, 1),
	}
}

func getMissingNodesAndEdges(s *simulationState, g *model.Graph) ([]*model.Node, []*model.Edge) {
	if s.modelToLayoutNodeLookup == nil || s.modelToLayoutEdgeLookup == nil {
		return []*model.Node{}, []*model.Edge{}
	}
	missingNodes := []*model.Node{}
	for _, node := range g.Nodes {
		if _, exists := s.modelToLayoutNodeLookup[node.ID]; !exists {
			missingNodes = append(missingNodes, node)
		}
	}
	missingEdges := []*model.Edge{}
	for _, edge := range g.Edges {
		if _, exists := s.modelToLayoutEdgeLookup[edge.ID]; !exists {
			missingEdges = append(missingEdges, edge)
		}
	}
	return missingNodes, missingEdges
}

func copyState(s *simulationState) *simulationState {
	p := simulationState{
		lnodes:                  make([]*layout.Node, len(s.lnodes)),
		ledges:                  make([]*layout.Edge, len(s.ledges)),
		modelToLayoutNodeLookup: make(map[string]int, len(s.modelToLayoutEdgeLookup)),
		modelToLayoutEdgeLookup: make(map[string]int, len(s.modelToLayoutEdgeLookup)),
	}
	for i := range s.lnodes {
		p.lnodes[i] = &layout.Node{}
		*p.lnodes[i] = *s.lnodes[i]
	}
	for i := range s.ledges {
		p.ledges[i] = &layout.Edge{}
		*p.ledges[i] = *s.ledges[i]
	}
	for k, v := range s.modelToLayoutNodeLookup {
		p.modelToLayoutNodeLookup[k] = v
	}
	for k, v := range s.modelToLayoutEdgeLookup {
		p.modelToLayoutEdgeLookup[k] = v
	}
	return &p
}

func (l *ForceSimulationLayouter) GetNodePositions(ctx context.Context, g *model.Graph) {
	select {
	case <-l.waitForInitialLayout:
	}
	s := l.simulationState
	missingNodes, missingEdges := getMissingNodesAndEdges(s, g)
	if len(missingNodes) > 0 || len(missingEdges) > 0 {
		// use a copy of the state for the quick simulation
		s = copyState(l.simulationState)
		for _, node := range s.lnodes {
			node.IsPinned = true
		}
		newNodes, _ := appendNodesAndEdges(s, missingNodes, missingEdges)
		l.quickSimulation.InitializeNodes(ctx, newNodes)                     // initialize only new nodes
		_, stats := l.quickSimulation.ComputeLayout(ctx, s.lnodes, s.ledges) // run quickSimulation with all nodes & edges
		l.updateGraphWithPositions(s, g)
		log.Info().Msgf(
			"*quick* graph layout computaton finished: stats{iterations: %d, time: %d ms}",
			stats.Iterations,
			stats.TotalTime.Milliseconds(),
		)
	}
	for i, node := range g.Nodes {
		idx := s.modelToLayoutNodeLookup[node.ID]
		node := s.lnodes[idx]
		g.Nodes[i].Position = &model.Vector{
			X: node.Pos.X(),
			Y: node.Pos.Y(),
			Z: node.Pos.Z(),
		}
	}
}

// TODO(skep): use a  onChange channel in the Controller -> Reload should always run the graph embedding if called!
func (l *ForceSimulationLayouter) shouldRun(g *model.Graph) bool {
	s := l.simulationState
	if s.modelToLayoutNodeLookup == nil || s.modelToLayoutEdgeLookup == nil {
		return true // initial run
	}
	missingNodes, missingEdges := getMissingNodesAndEdges(s, g)
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
	s := simulationState{}
	s.lnodes, s.ledges = []*layout.Node{}, []*layout.Edge{}
	s.modelToLayoutNodeLookup = make(map[string]int, len(g.Nodes))
	s.modelToLayoutEdgeLookup = make(map[string]int, len(g.Edges))
	appendNodesAndEdges(&s, g.Nodes, g.Edges)
	l.completeSimulation.InitializeNodes(ctx, s.lnodes)
	_, stats := l.completeSimulation.ComputeLayout(ctx, s.lnodes, s.ledges)
	l.updateGraphWithPositions(&s, g)
	l.simulationState = &s
	if !l.initialLayoutDone {
		l.initialLayoutDone = true
		close(l.waitForInitialLayout)
	}
	return stats
}

func (l *ForceSimulationLayouter) updateGraphWithPositions(s *simulationState, g *model.Graph) {
	for i := range g.Nodes {
		idx := s.modelToLayoutNodeLookup[g.Nodes[i].ID]
		node := s.lnodes[idx]
		g.Nodes[i].Position = &model.Vector{X: node.Pos.X(), Y: node.Pos.Y(), Z: node.Pos.Z()}
	}
}

// returns newly added nodes and edges as layout.{Node/Edge} type
func appendNodesAndEdges(s *simulationState, nodes []*model.Node, edges []*model.Edge) ([]*layout.Node, []*layout.Edge) {
	newNodes := []*layout.Node{}
	for index, node := range nodes {
		newNodes = append(newNodes, &layout.Node{Name: node.Description})
		s.modelToLayoutNodeLookup[node.ID] = index + len(s.lnodes)
	}
	newEdges := []*layout.Edge{}
	for index, edge := range edges {
		newEdges = append(newEdges, &layout.Edge{Source: s.modelToLayoutNodeLookup[edge.From], Target: s.modelToLayoutNodeLookup[edge.To]})
		s.modelToLayoutEdgeLookup[edge.ID] = index + len(s.ledges)
	}
	s.lnodes = append(s.lnodes, newNodes...)
	s.ledges = append(s.ledges, newEdges...)
	return newNodes, newEdges
}

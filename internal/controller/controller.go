package controller

import (
	"context"
	"errors"
	"runtime"

	"github.com/rs/zerolog/log"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
	"github.com/suxatcode/learn-graph-poc-backend/layout"
	"github.com/suxatcode/learn-graph-poc-backend/middleware"
)

const (
	AuthNeededForGraphDataChangeMsg = `only logged in user may create graph data`
)

var (
	AuthNeededForGraphDataChangeErr    = errors.New(AuthNeededForGraphDataChangeMsg)
	AuthNeededForGraphDataChangeStatus = &model.Status{Message: AuthNeededForGraphDataChangeMsg}
	AuthNeededForGraphDataChangeResult = &model.CreateEntityResult{Status: AuthNeededForGraphDataChangeStatus}
)

type Controller struct {
	db db.DB
}

func NewController(newdb db.DB) *Controller {
	return &Controller{db: newdb}
}

func (c *Controller) CreateNode(ctx context.Context, description model.Text, resources *model.Text) (*model.CreateEntityResult, error) {
	authenticated, user, err := c.db.IsUserAuthenticated(ctx)
	if err != nil || !authenticated || user == nil {
		if err != nil {
			log.Ctx(ctx).Error().Msgf("%v", err)
			return nil, err
		}
		log.Ctx(ctx).Error().Msgf("user '%s' (token '%s') not authenticated", middleware.CtxGetUserID(ctx), middleware.CtxGetAuthentication(ctx))
		return AuthNeededForGraphDataChangeResult, AuthNeededForGraphDataChangeErr
	}
	id, err := c.db.CreateNode(ctx, *user, &description, resources)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("%v", err)
		return nil, err
	}
	res := &model.CreateEntityResult{ID: id}
	log.Ctx(ctx).Debug().Msgf("CreateNode() -> %v", res)
	return res, nil
}

func (c *Controller) CreateEdge(ctx context.Context, from string, to string, weight float64) (*model.CreateEntityResult, error) {
	authenticated, user, err := c.db.IsUserAuthenticated(ctx)
	if err != nil || !authenticated || user == nil {
		if err != nil {
			log.Ctx(ctx).Error().Msgf("%v", err)
			return nil, err
		}
		log.Ctx(ctx).Error().Msgf("user '%s' (token '%s') not authenticated", middleware.CtxGetUserID(ctx), middleware.CtxGetAuthentication(ctx))
		return AuthNeededForGraphDataChangeResult, AuthNeededForGraphDataChangeErr
	}
	ID, err := c.db.CreateEdge(ctx, *user, from, to, weight)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("%v", err)
		return nil, err
	}
	res := &model.CreateEntityResult{ID: ID}
	log.Ctx(ctx).Debug().Msgf("CreateEdge() -> %v", res)
	return res, nil
}

func (c *Controller) EditNode(ctx context.Context, id string, description model.Text, resources *model.Text) (*model.Status, error) {
	authenticated, user, err := c.db.IsUserAuthenticated(ctx)
	if err != nil || !authenticated || user == nil {
		if err != nil {
			log.Ctx(ctx).Error().Msgf("%v", err)
			return nil, err
		}
		log.Ctx(ctx).Error().Msgf("user '%s' (token '%s') not authenticated", middleware.CtxGetUserID(ctx), middleware.CtxGetAuthentication(ctx))
		return AuthNeededForGraphDataChangeStatus, AuthNeededForGraphDataChangeErr
	}
	err = c.db.EditNode(ctx, *user, id, &description, resources)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("%v", err)
		return nil, err
	}
	log.Ctx(ctx).Debug().Msgf("EditNode() -> %v", nil)
	return nil, nil
}

func (c *Controller) SubmitVote(ctx context.Context, id string, value float64) (*model.Status, error) {
	authenticated, user, err := c.db.IsUserAuthenticated(ctx)
	if err != nil || !authenticated || user == nil {
		if err != nil {
			log.Ctx(ctx).Error().Msgf("%v", err)
			return nil, err
		}
		log.Ctx(ctx).Error().Msgf("user '%s' (token '%s') not authenticated", middleware.CtxGetUserID(ctx), middleware.CtxGetAuthentication(ctx))
		return AuthNeededForGraphDataChangeStatus, AuthNeededForGraphDataChangeErr
	}
	err = c.db.AddEdgeWeightVote(ctx, *user, id, value)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("%v", err)
		return nil, err
	}
	log.Ctx(ctx).Debug().Msgf("SubmitVote() -> %v", nil)
	return nil, nil
}

func AddPreComputedNodePositions(ctx context.Context, g *model.Graph) {
	max_y := 1000.0
	fs := layout.NewForceSimulation(layout.ForceSimulationConfig{
		AlphaDecay: 0.005,
		Rect:       layout.Rect{X: 0.0, Y: 0.0, Width: max_y * 2, Height: max_y}, ScreenMultiplierToClampPosition: 1000,
		Parallelization: runtime.NumCPU(),
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
	log.Ctx(ctx).Info().Msgf("graph layout: {iterations: %d, time: %d ms}", stats.Iterations, stats.TotalTime.Milliseconds())
}

func (c *Controller) Graph(ctx context.Context) (*model.Graph, error) {
	// TODO(skep): refactor graph handling
	//	2. use node.IDs here for force simulation: create intermediary layout.Graph type
	//  3. add positions to the returned model.Graph type
	g, err := c.db.Graph(ctx)
	if err != nil || g == nil {
		log.Ctx(ctx).Error().Msgf("%v | graph=%v", err, g)
	} else if g != nil {
		log.Ctx(ctx).Debug().Msgf("returns %d nodes and %d edges", len(g.Nodes), len(g.Edges))
	}
	AddPreComputedNodePositions(ctx, g)
	return g, err
}

func (c *Controller) DeleteNode(ctx context.Context, id string) (*model.Status, error) {
	authenticated, user, err := c.db.IsUserAuthenticated(ctx)
	if err != nil || !authenticated || user == nil {
		if err != nil {
			log.Ctx(ctx).Error().Msgf("%v", err)
			return nil, err
		}
		log.Ctx(ctx).Error().Msgf("user '%s' (token '%s') not authenticated", middleware.CtxGetUserID(ctx), middleware.CtxGetAuthentication(ctx))
		return AuthNeededForGraphDataChangeStatus, AuthNeededForGraphDataChangeErr
	}
	err = c.db.DeleteNode(ctx, *user, id)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("%v", err)
		return nil, err
	}
	log.Ctx(ctx).Debug().Msgf("DeleteNode() -> %v", nil)
	return nil, nil
}

func (c *Controller) DeleteEdge(ctx context.Context, id string) (*model.Status, error) {
	authenticated, user, err := c.db.IsUserAuthenticated(ctx)
	if err != nil || !authenticated || user == nil {
		if err != nil {
			log.Ctx(ctx).Error().Msgf("%v", err)
			return nil, err
		}
		log.Ctx(ctx).Error().Msgf("user '%s' (token '%s') not authenticated", middleware.CtxGetUserID(ctx), middleware.CtxGetAuthentication(ctx))
		return AuthNeededForGraphDataChangeStatus, AuthNeededForGraphDataChangeErr
	}
	err = c.db.DeleteEdge(ctx, *user, id)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("%v", err)
		return nil, err
	}
	log.Ctx(ctx).Debug().Msgf("DeleteEdge() -> %v", nil)
	return nil, nil
}

func (c *Controller) NodeEdits(ctx context.Context, id string) ([]*model.NodeEdit, error) {
	edits, err := c.db.NodeEdits(ctx, id)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("%v", err)
		return nil, err
	}
	log.Ctx(ctx).Debug().Msgf("NodeEdits() -> %v", edits)
	return edits, nil
}

func (c *Controller) EdgeEdits(ctx context.Context, id string) ([]*model.EdgeEdit, error) {
	edits, err := c.db.EdgeEdits(ctx, id)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("%v", err)
		return nil, err
	}
	log.Ctx(ctx).Debug().Msgf("EdgeEdits() -> %v", edits)
	return edits, nil
}

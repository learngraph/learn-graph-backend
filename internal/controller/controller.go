package controller

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
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
	db       db.DB
	layouter Layouter
}

func NewController(newdb db.DB, newlayouter Layouter) *Controller {
	return &Controller{db: newdb, layouter: newlayouter} // TODO(erem): put layouter in here
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

func (c *Controller) Graph(ctx context.Context) (*model.Graph, error) {
	g, err := c.db.Graph(ctx)
	if err != nil || g == nil {
		log.Ctx(ctx).Error().Msgf("%v | graph=%v", err, g)
	} else if g != nil {
		c.layouter.GetNodePositions(ctx, g)
		log.Ctx(ctx).Debug().Msgf("Graph() returns %d nodes and %d edges", len(g.Nodes), len(g.Edges))
	}
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

// PeriodicGraphEmbeddingComputation periodically calls c.layouter.Reload() to
// re-compute the graph embedding.
func (c *Controller) PeriodicGraphEmbeddingComputation(ctx context.Context) {
	recomputationInterval := time.Second * 60
	ticker := time.NewTicker(recomputationInterval)
	defer ticker.Stop()
	graph := func(ctx context.Context) *model.Graph {
		g, err := c.db.Graph(ctx)
		if err != nil || g == nil {
			log.Ctx(ctx).Err(err).Msg("failed to fetch graph from db for embedding computation")
		}
		return g
	}
	reload := func(ctx context.Context, g *model.Graph) {
		stats := c.layouter.Reload(ctx, g)
		if stats.Iterations == 0 {
			// no graph embedding happened, probably nothing new to compute
			return
		}
		log.Info().Msgf(
			"periodic graph layout computaton finished: stats{iterations: %d, time: %d ms}",
			stats.Iterations,
			stats.TotalTime.Milliseconds(),
		)
	}
	{
		// perform layouting once initially
		initCtx, cancelInit := context.WithTimeout(ctx, recomputationInterval/2)
		if g := graph(initCtx); g != nil {
			reload(initCtx, g)
		}
		cancelInit()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reloadCtx, cancelReload := context.WithTimeout(ctx, recomputationInterval/2)
			if g := graph(reloadCtx); g != nil {
				reload(reloadCtx, g)
			}
			cancelReload()
		}
	}
}

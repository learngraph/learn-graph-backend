package controller

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
	"github.com/suxatcode/learn-graph-poc-backend/layout"
)

var (
	user444 = db.User{Document: db.Document{Key: "444"}}
)

func TestController_CreateNode(t *testing.T) {
	for _, test := range []struct {
		Name             string
		MockExpectations func(context.Context, db.MockDB)
		ExpectRes        *model.CreateEntityResult
		ExpectErr        bool
		Description      model.Text
	}{
		{
			Name: "user authenticated, node created",
			MockExpectations: func(ctx context.Context, mock db.MockDB) {
				mock.EXPECT().IsUserAuthenticated(gomock.Any()).Return(true, &user444, nil)
				mock.EXPECT().CreateNode(ctx, user444, &model.Text{Translations: []*model.Translation{
					{Language: "en", Content: "ok"},
				}}, nil).Return("123", nil)
			},
			Description: model.Text{Translations: []*model.Translation{
				{Language: "en", Content: "ok"},
			}},
			ExpectRes: &model.CreateEntityResult{ID: "123", Status: nil},
		},
		{
			Name: "user not authenticated, no node created",
			MockExpectations: func(ctx context.Context, mock db.MockDB) {
				mock.EXPECT().IsUserAuthenticated(gomock.Any()).Return(false, nil, nil)
			},
			ExpectRes: &model.CreateEntityResult{ID: "", Status: &model.Status{Message: "only logged in user may create graph data"}},
			ExpectErr: true,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			t.Log(test.Name)
			ctrl := gomock.NewController(t)
			db := db.NewMockDB(ctrl)
			ctx := context.Background()
			test.MockExpectations(ctx, *db)
			c := NewController(db, nil)
			id, err := c.CreateNode(ctx, test.Description, nil)
			assert := assert.New(t)
			assert.Equal(test.ExpectRes, id)
			if test.ExpectErr {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}

func TestController_EditNode(t *testing.T) {
	for _, test := range []struct {
		Name             string
		MockExpectations func(context.Context, db.MockDB)
		ExpectRes        *model.Status
		ExpectErr        bool
		Description      model.Text
		NodeID           string
	}{
		{
			Name: "user authenticated, node edited",
			MockExpectations: func(ctx context.Context, mock db.MockDB) {
				mock.EXPECT().IsUserAuthenticated(gomock.Any()).Return(true, &user444, nil)
				mock.EXPECT().EditNode(ctx, user444, "123", &model.Text{Translations: []*model.Translation{
					{Language: "en", Content: "ok"},
				}}, nil).Return(nil)
			},
			Description: model.Text{Translations: []*model.Translation{
				{Language: "en", Content: "ok"},
			}},
			NodeID: "123",
		},
		{
			Name: "user not authenticated, node not edited",
			MockExpectations: func(ctx context.Context, mock db.MockDB) {
				mock.EXPECT().IsUserAuthenticated(gomock.Any()).Return(false, nil, nil)
			},
			Description: model.Text{Translations: []*model.Translation{
				{Language: "en", Content: "ok"},
			}},
			NodeID:    "123",
			ExpectErr: true,
			ExpectRes: AuthNeededForGraphDataChangeStatus,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			t.Log(test.Name)
			ctrl := gomock.NewController(t)
			db := db.NewMockDB(ctrl)
			ctx := context.Background()
			test.MockExpectations(ctx, *db)
			c := NewController(db, nil)
			status, err := c.EditNode(ctx, test.NodeID, test.Description, nil)
			assert := assert.New(t)
			assert.Equal(test.ExpectRes, status)
			if test.ExpectErr {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}

func TestController_EditNode_ShouldAlwaysLogOnError(t *testing.T) {
	for _, test := range []struct {
		Name             string
		MockExpectations func(context.Context, db.MockDB)
		LogContains      string
		ExpectedStatus   *model.Status
	}{
		{
			Name: "no auth, no error",
			MockExpectations: func(ctx context.Context, mock db.MockDB) {
				mock.EXPECT().IsUserAuthenticated(gomock.Any()).Return(false, nil, nil)
			},
			LogContains:    `not authenticated`,
			ExpectedStatus: AuthNeededForGraphDataChangeStatus,
		},
		{
			Name: "no auth, with error",
			MockExpectations: func(ctx context.Context, mock db.MockDB) {
				mock.EXPECT().IsUserAuthenticated(gomock.Any()).Return(false, nil, errors.New(`AAA`))
			},
			LogContains:    `AAA`,
			ExpectedStatus: nil,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			t.Log(test.Name)
			logBuffer := bytes.NewBuffer([]byte{})
			logger := zerolog.New(logBuffer).Level(zerolog.ErrorLevel).With().Str("test", "EditNode").Logger()
			ctx := logger.WithContext(context.Background())
			ctrl := gomock.NewController(t)
			db := db.NewMockDB(ctrl)
			test.MockExpectations(ctx, *db)
			c := NewController(db, nil)
			status, err := c.EditNode(ctx, "123", model.Text{Translations: []*model.Translation{{Language: "en", Content: "ok"}}}, nil)
			assert := assert.New(t)
			assert.Equal(test.ExpectedStatus, status)
			assert.Error(err)
			assert.Contains(logBuffer.String(), test.LogContains)
		})
	}
}

func TestController_SubmitVote(t *testing.T) {
	for _, test := range []struct {
		Name             string
		MockExpectations func(context.Context, db.MockDB)
		ExpectRes        *model.Status
		ExpectErr        bool
		NodeID           string
		Value            float64
	}{
		{
			Name: "user authenticated, node edited",
			MockExpectations: func(ctx context.Context, mock db.MockDB) {
				mock.EXPECT().IsUserAuthenticated(gomock.Any()).Return(true, &user444, nil)
				mock.EXPECT().AddEdgeWeightVote(ctx, user444, "123", 1.1).Return(nil)
			},
			NodeID: "123",
			Value:  1.1,
		},
		{
			Name: "user not authenticated, node not edited",
			MockExpectations: func(ctx context.Context, mock db.MockDB) {
				mock.EXPECT().IsUserAuthenticated(gomock.Any()).Return(false, nil, nil)
			},
			NodeID:    "123",
			Value:     1.1,
			ExpectErr: true,
			ExpectRes: AuthNeededForGraphDataChangeStatus,
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			t.Log(test.Name)
			ctrl := gomock.NewController(t)
			db := db.NewMockDB(ctrl)
			ctx := context.Background()
			test.MockExpectations(ctx, *db)
			c := NewController(db, nil)
			status, err := c.SubmitVote(ctx, test.NodeID, test.Value)
			assert := assert.New(t)
			assert.Equal(test.ExpectRes, status)
			if test.ExpectErr {
				assert.Error(err)
			} else {
				assert.NoError(err)
			}
		})
	}
}

func TestController_DeleteNode(t *testing.T) {
	for _, test := range []struct {
		Name             string
		MockExpectations func(context.Context, db.MockDB)
		ExpectRes        *model.Status
		ExpectErr        bool
	}{
		{
			Name: "user authenticated, node created",
			MockExpectations: func(ctx context.Context, mock db.MockDB) {
				mock.EXPECT().IsUserAuthenticated(gomock.Any()).Return(true, &user444, nil)
				mock.EXPECT().DeleteNode(ctx, user444, "123").Return(nil)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := db.NewMockDB(ctrl)
			ctx := context.Background()
			test.MockExpectations(ctx, *db)
			c := NewController(db, nil)
			status, err := c.DeleteNode(ctx, "123")
			assert := assert.New(t)
			assert.Equal(test.ExpectRes, status)
			if !test.ExpectErr {
				assert.NoError(err)
			} else {
				assert.Error(err)
			}
		})
	}
}

func TestController_DeleteEdge(t *testing.T) {
	for _, test := range []struct {
		Name             string
		MockExpectations func(context.Context, db.MockDB)
		ExpectRes        *model.Status
		ExpectErr        bool
	}{
		{
			Name: "user authenticated, node created",
			MockExpectations: func(ctx context.Context, mock db.MockDB) {
				mock.EXPECT().IsUserAuthenticated(gomock.Any()).Return(true, &user444, nil)
				mock.EXPECT().DeleteEdge(ctx, user444, "123").Return(nil)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := db.NewMockDB(ctrl)
			ctx := context.Background()
			test.MockExpectations(ctx, *db)
			c := NewController(db, nil)
			status, err := c.DeleteEdge(ctx, "123")
			assert := assert.New(t)
			assert.Equal(test.ExpectRes, status)
			if !test.ExpectErr {
				assert.NoError(err)
			} else {
				assert.Error(err)
			}
		})
	}
}

func TestController_NodeEdits(t *testing.T) {
	for _, test := range []struct {
		Name             string
		MockExpectations func(context.Context, db.MockDB)
		ExpectRes        []*model.NodeEdit
		ExpectErr        bool
	}{
		{
			Name: "single Node Edit",
			MockExpectations: func(ctx context.Context, mock db.MockDB) {
				mock.EXPECT().NodeEdits(ctx, "123").Return([]*model.NodeEdit{{
					Username: "Me Me",
				}},
					nil)
			},
			ExpectRes: []*model.NodeEdit{{
				Username: "Me Me",
			}},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := db.NewMockDB(ctrl)
			ctx := context.Background()
			test.MockExpectations(ctx, *db)
			c := NewController(db, nil)
			edits, err := c.NodeEdits(ctx, "123")
			assert := assert.New(t)
			assert.Equal(test.ExpectRes, edits)
			if !test.ExpectErr {
				assert.NoError(err)
			} else {
				assert.Error(err)
			}
		})
	}
}

func TestController_EdgeEdits(t *testing.T) {
	for _, test := range []struct {
		Name             string
		MockExpectations func(context.Context, db.MockDB)
		ExpectRes        []*model.EdgeEdit
		ExpectErr        bool
	}{
		{
			Name: "single Edge Edit",
			MockExpectations: func(ctx context.Context, mock db.MockDB) {
				mock.EXPECT().EdgeEdits(ctx, "123").Return([]*model.EdgeEdit{{
					Username: "Me Me",
				}},
					nil)
			},
			ExpectRes: []*model.EdgeEdit{{
				Username: "Me Me",
			}},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := db.NewMockDB(ctrl)
			ctx := context.Background()
			test.MockExpectations(ctx, *db)
			c := NewController(db, nil)
			edits, err := c.EdgeEdits(ctx, "123")
			assert := assert.New(t)
			assert.Equal(test.ExpectRes, edits)
			if !test.ExpectErr {
				assert.NoError(err)
			} else {
				assert.Error(err)
			}
		})
	}
}

func TestController_Graph(t *testing.T) {
	for _, test := range []struct {
		Name             string
		MockExpectations func(context.Context, db.MockDB, MockLayouter)
		ExpectGraph      *model.Graph
		ExpectRes        *model.Status
		ExpectErr        bool
	}{
		{
			Name:        "assume added positions",
			ExpectGraph: &model.Graph{Nodes: []*model.Node{{Position: &model.Vector{X: 1, Y: 2, Z: 3}}}},
			MockExpectations: func(ctx context.Context, mockDB db.MockDB, mockLayouter MockLayouter) {
				mockDB.EXPECT().Graph(ctx).Return(&model.Graph{
					Nodes: []*model.Node{{}},
				}, nil)
				mockLayouter.EXPECT().GetNodePositions(ctx, gomock.Eq(&model.Graph{
					Nodes: []*model.Node{{}},
				})).DoAndReturn(
					func(ctx context.Context, g *model.Graph) {
						g.Nodes[0].Position = &model.Vector{X: 1, Y: 2, Z: 3}
					},
				)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := db.NewMockDB(ctrl)
			l := NewMockLayouter(ctrl)
			ctx := context.Background()
			test.MockExpectations(ctx, *db, *l)
			c := NewController(db, l)
			graph, err := c.Graph(ctx)
			assert := assert.New(t)
			if test.ExpectErr {
				assert.Error(err)
			} else {
				if !assert.NoError(err) {
					return
				}
			}
			assert.Equal(test.ExpectGraph, graph)
		})
	}
}

func TestController_periodicGraphEmbeddingComputation(t *testing.T) {
	for _, test := range []struct {
		Name             string
		MockExpectations func(context.Context, db.MockDB, MockLayouter)
		Setup            func(trigger chan time.Time)
	}{
		{
			Name: "should run layout on startup",
			MockExpectations: func(ctx context.Context, mockDB db.MockDB, mockLayouter MockLayouter) {
				mockDB.EXPECT().Graph(gomock.Any()).Return(&model.Graph{Nodes: []*model.Node{{}, {}}}, nil)
				mockLayouter.EXPECT().Reload(gomock.Any(), &model.Graph{Nodes: []*model.Node{{}, {}}}).Return(layout.Stats{Iterations: 5})
			},
		},
		{
			Name: "should run layout on trigger call",
			MockExpectations: func(ctx context.Context, mockDB db.MockDB, mockLayouter MockLayouter) {
				mockDB.EXPECT().Graph(gomock.Any()).Return(&model.Graph{Nodes: []*model.Node{{}, {}}}, nil)
				mockLayouter.EXPECT().Reload(gomock.Any(), &model.Graph{Nodes: []*model.Node{{}, {}}}).Return(layout.Stats{Iterations: 5})
				// 2nd call
				mockDB.EXPECT().Graph(gomock.Any()).Return(&model.Graph{Nodes: []*model.Node{{}, {}}}, nil)
				mockLayouter.EXPECT().Reload(gomock.Any(), &model.Graph{Nodes: []*model.Node{{}, {}}}).Return(layout.Stats{Iterations: 5})
			},
			Setup: func(trigger chan time.Time) {
				trigger <- time.UnixMilli(7)
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			db := db.NewMockDB(ctrl)
			l := NewMockLayouter(ctrl)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			test.MockExpectations(ctx, *db, *l)
			c := NewController(db, l)
			trigger := make(chan time.Time, 10)
			if test.Setup != nil {
				test.Setup(trigger)
			}
			go c.periodicGraphEmbeddingComputation(ctx, trigger, time.Second*1)
			time.Sleep(time.Millisecond * 10) // XXX(skep): could be a flaky test some day: can we do it without sleeping
		})
	}
}

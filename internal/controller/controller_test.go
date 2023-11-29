package controller

import (
	"bytes"
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
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
				}}).Return("123", nil)
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
			c := NewController(db)
			id, err := c.CreateNode(ctx, test.Description)
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
				}}).Return(nil)
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
			c := NewController(db)
			status, err := c.EditNode(ctx, test.NodeID, test.Description)
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
			c := NewController(db)
			status, err := c.EditNode(ctx, "123", model.Text{Translations: []*model.Translation{{Language: "en", Content: "ok"}}})
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
				mock.EXPECT().SetEdgeWeight(ctx, user444, "123", 1.1).Return(nil)
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
			c := NewController(db)
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

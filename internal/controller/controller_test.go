package controller

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
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

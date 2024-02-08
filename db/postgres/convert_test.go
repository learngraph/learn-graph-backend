package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
	"gorm.io/gorm"
)

func TestConvertToModelGraph(t *testing.T) {
	for _, test := range []struct {
		Name     string
		InpV     []Node
		InpE     []Edge
		Language string
		Exp      *model.Graph
	}{
		{
			Name:     "single node",
			InpV:     []Node{{Model: gorm.Model{ID: 123}, Description: db.Text{"en": "a"}}},
			Language: "en",
			Exp: &model.Graph{
				Nodes: []*model.Node{
					{ID: "123", Description: "a"},
				},
			},
		},
		{
			Name: "multiple nodes",
			InpV: []Node{
				{Model: gorm.Model{ID: 123}, Description: db.Text{"en": "a"}},
				{Model: gorm.Model{ID: 456}, Description: db.Text{"en": "a"}},
			},
			Language: "en",
			Exp: &model.Graph{
				Nodes: []*model.Node{
					{ID: "123", Description: "a"},
					{ID: "456", Description: "a"},
				},
			},
		},
		{
			Name: "2 nodes 1 edge",
			InpV: []Node{
				{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "a"}},
				{Model: gorm.Model{ID: 2}, Description: db.Text{"en": "b"}},
			},
			InpE: []Edge{
				{Model: gorm.Model{ID: 3}, FromID: 1, ToID: 2},
			},
			Language: "en",
			Exp: &model.Graph{
				Nodes: []*model.Node{
					{ID: "1", Description: "a"},
					{ID: "2", Description: "b"},
				},
				Edges: []*model.Edge{
					{ID: "3", From: "1", To: "2"},
				},
			},
		},
		{
			Name:     "single node, only requested description language, should use FallbackLanguage",
			InpV:     []Node{{Model: gorm.Model{ID: 1}, Description: db.Text{"en": "ok"}}},
			Language: "ch",
			Exp: &model.Graph{
				Nodes: []*model.Node{
					{ID: "1", Description: "ok"},
				},
			},
		},
		{
			Name:     "single node, description missing, should skip node",
			InpV:     []Node{{Model: gorm.Model{ID: 1}, Description: db.Text{}}},
			Language: "en",
			Exp:      &model.Graph{},
		},
		{
			Name:     "single node, only foreign description, should display foreign language",
			InpV:     []Node{{Model: gorm.Model{ID: 1}, Description: db.Text{"zh": "打坐"}}},
			Language: "en",
			Exp: &model.Graph{
				Nodes: []*model.Node{
					{ID: "1", Description: "打坐"},
				},
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.Exp, NewConvertToModel(test.Language).Graph(test.InpV, test.InpE))
		})
	}
}

func TestConvertToDBText(t *testing.T) {
	for _, test := range []struct {
		Name string
		Inp  *model.Text
		Exp  db.Text
	}{
		{
			Name: "empty translations should return empty map (not nil)",
			Inp: &model.Text{
				Translations: []*model.Translation{},
			},
			Exp: db.Text{},
		},
		{
			Name: "non-empty, but nil translations should return empty map",
			Inp: &model.Text{
				Translations: []*model.Translation{nil, nil},
			},
			Exp: db.Text{},
		},
		{
			Name: "2 entries",
			Inp: &model.Text{
				Translations: []*model.Translation{
					{
						Language: "a",
						Content:  "b",
					},
					{
						Language: "c",
						Content:  "d",
					},
				},
			},
			Exp: db.Text{
				"a": "b",
				"c": "d",
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.Exp, ConvertToDBText(test.Inp))
		})
	}
}

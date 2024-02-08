package arangodb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

func TestConvertToModelGraph(t *testing.T) {
	for _, test := range []struct {
		Name     string
		InpV     []db.Node
		InpE     []db.Edge
		Language string
		Exp      *model.Graph
	}{
		{
			Name:     "single node",
			InpV:     []db.Node{{Document: db.Document{Key: "abc"}, Description: db.Text{"en": "a"}}},
			Language: "en",
			Exp: &model.Graph{
				Nodes: []*model.Node{
					{ID: "abc", Description: "a"},
				},
			},
		},
		{
			Name: "multiple nodes",
			InpV: []db.Node{
				{Document: db.Document{Key: "abc"}, Description: db.Text{"en": "a"}},
				{Document: db.Document{Key: "def"}, Description: db.Text{"en": "a"}},
			},
			Language: "en",
			Exp: &model.Graph{
				Nodes: []*model.Node{
					{ID: "abc", Description: "a"},
					{ID: "def", Description: "a"},
				},
			},
		},
		{
			Name: "2 nodes 1 edge",
			InpV: []db.Node{
				{Document: db.Document{Key: "a"}, Description: db.Text{"en": "a"}},
				{Document: db.Document{Key: "b"}, Description: db.Text{"en": "b"}},
			},
			InpE: []db.Edge{
				{Document: db.Document{Key: "?"}, From: "nodes/a", To: "nodes/b"},
			},
			Language: "en",
			Exp: &model.Graph{
				Nodes: []*model.Node{
					{ID: "a", Description: "a"},
					{ID: "b", Description: "b"},
				},
				Edges: []*model.Edge{
					{ID: "?", From: "a", To: "b"},
				},
			},
		},
		{
			Name: "invalid node-key in edge: should skip edge",
			InpV: []db.Node{
				{Document: db.Document{Key: "a"}, Description: db.Text{"en": "a"}},
				{Document: db.Document{Key: "b"}, Description: db.Text{"en": "b"}},
			},
			InpE: []db.Edge{
				{Document: db.Document{Key: "e"}, From: "nods/a", To: "nods/b"},
			},
			Language: "en",
			Exp: &model.Graph{
				Nodes: []*model.Node{
					{ID: "a", Description: "a"},
					{ID: "b", Description: "b"},
				},
			},
		},
		{
			Name:     "single node, only requested description language, should use FallbackLanguage with added Flag",
			InpV:     []db.Node{{Document: db.Document{Key: "abc"}, Description: db.Text{"en": "ok"}}},
			Language: "ch",
			Exp: &model.Graph{
				Nodes: []*model.Node{
					{ID: "abc", Description: "🇺🇸 ok"},
				},
			},
		},
		{
			Name:     "single node, description missing, should skip node",
			InpV:     []db.Node{{Document: db.Document{Key: "abc"}, Description: db.Text{}}},
			Language: "en",
			Exp:      &model.Graph{},
		},
		{
			Name:     "single node, only foreign description, should display foreign language with added Flag",
			InpV:     []db.Node{{Document: db.Document{Key: "abc"}, Description: db.Text{"zh": "對"}}},
			Language: "en",
			Exp: &model.Graph{
				Nodes: []*model.Node{
					{ID: "abc", Description: "🇹🇼 對"},
				},
			},
		},
		{
			Name:     "single node, only foreign resources",
			InpV:     []db.Node{{Document: db.Document{Key: "abc"}, Description: db.Text{"zh": "對"}, Resources: db.Text{"en": "A"}}},
			Language: "zh",
			Exp: &model.Graph{
				Nodes: []*model.Node{
					{ID: "abc", Description: "對", Resources: strptr("🇺🇸 A")},
				},
			},
		},
	} {
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, test.Exp, NewConvertToModel(test.Language).Graph(test.InpV, test.InpE))
		})
	}
}

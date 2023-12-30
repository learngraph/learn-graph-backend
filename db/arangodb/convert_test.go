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
			Name:     "single node, only requested description language, should use FallbackLanguage",
			InpV:     []db.Node{{Document: db.Document{Key: "abc"}, Description: db.Text{"en": "ok"}}},
			Language: "ch",
			Exp: &model.Graph{
				Nodes: []*model.Node{
					{ID: "abc", Description: "ok"},
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
			Name:     "single node, only foreign description, should skip node",
			InpV:     []db.Node{{Document: db.Document{Key: "abc"}, Description: db.Text{"ch": "ok"}}},
			Language: "en",
			Exp:      &model.Graph{},
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

//func TestMergeText(t *testing.T) {
//	for _, test := range []struct {
//		Name                 string
//		Basis, Override, Exp Text
//	}{
//		{
//			Name:     "simple addition",
//			Basis:    db.Text{"a": "1"},
//			Override: db.Text{"b": "2"},
//			Exp:      db.Text{"a": "1", "b": "2"},
//		},
//		{
//			Name:     "override",
//			Basis:    db.Text{"a": "1"},
//			Override: db.Text{"a": "2"},
//			Exp:      db.Text{"a": "2"},
//		},
//		{
//			Name:     "nil input",
//			Basis:    nil,
//			Override: nil,
//			Exp:      db.Text{},
//		},
//	} {
//		t.Run(test.Name, func(t *testing.T) {
//			assert.Equal(t, test.Exp, MergeText(test.Basis, test.Override))
//		})
//	}
//}

package arangodb

import (
	"fmt"
	"strings"

	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

type ConvertToModel struct {
	language string
}

func NewConvertToModel(language string) *ConvertToModel {
	return &ConvertToModel{language: language}
}
func (c *ConvertToModel) Graph(nodes []db.Node, edges []db.Edge) *model.Graph {
	g := model.Graph{}
	for _, v := range nodes {
		description, ok := v.Description[c.language]
		if !ok {
			description, ok = v.Description[FallbackLanguage]
			if !ok {
				continue
			}
		}
		g.Nodes = append(g.Nodes, &model.Node{
			ID:          v.Key,
			Description: description,
		})
	}
	for _, e := range edges {
		g.Edges = append(g.Edges, &model.Edge{
			ID: e.Key,
			// TODO(skep): error handling!
			From:   strings.Split(e.From, nodePrefix)[1],
			To:     strings.Split(e.To, nodePrefix)[1],
			Weight: e.Weight,
		})
	}
	return &g
}

var nodePrefix = fmt.Sprintf("%s/", COLLECTION_NODES)

const FallbackLanguage = "en"

// maybe:
//type ConvertToDB struct{}
//
//func NewConvertToDB() *ConvertToDB {
//	return &ConvertToDB{}
//}
//func (c *ConvertToDB) Text(text *model.Text) Text

func ConvertToDBText(text *model.Text) db.Text {
	if text == nil {
		return db.Text{}
	}
	t := make(map[string]string, len(text.Translations))
	for _, translation := range text.Translations {
		if translation == nil {
			continue
		}
		t[translation.Language] = translation.Content
	}
	return t
}

//// basis is overridden with override if the same language exists in both texts
//func MergeText(basis, override Text) Text {
//	out := Text{}
//	for key, val := range basis {
//		out[key] = val
//	}
//	for key, val := range override {
//		out[key] = val
//	}
//	return out
//}

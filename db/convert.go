package db

import (
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

type ConvertToModel struct {
	language string
}

func NewConvertToModel(language string) *ConvertToModel {
	return &ConvertToModel{language: language}
}
func (c *ConvertToModel) Graph(nodes []Node, edges []Edge) *model.Graph {
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
			ID:     e.Key,
			From:   e.From,
			To:     e.To,
			Weight: e.Weight,
		})
	}
	return &g
}

const FallbackLanguage = "en"

// maybe:
//type ConvertToDB struct{}
//
//func NewConvertToDB() *ConvertToDB {
//	return &ConvertToDB{}
//}
//func (c *ConvertToDB) Text(text *model.Text) Text

func ConvertToDBText(text *model.Text) Text {
	if text == nil {
		return Text{}
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

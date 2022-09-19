package db

import (
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

func ModelFromDB(nodes []Node, edges []Edge) *model.Graph {
	g := model.Graph{}
	for _, v := range nodes {
		g.Nodes = append(g.Nodes, &model.Node{
			ID: v.Key,
		})
	}
	for _, e := range edges {
		g.Edges = append(g.Edges, &model.Edge{
			ID:   e.Key,
			From: e.From,
			To:   e.To,
		})
	}
	return &g
}

func ConvertTextToDB(text *model.Text) Text {
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

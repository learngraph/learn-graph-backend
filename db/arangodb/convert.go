package arangodb

import (
	"fmt"
	"strings"

	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

type ConvertToModel struct {
	language         string
	fallbackLanguage string
}

const defaultFallbackLanguage = "en"

func NewConvertToModel(language string) *ConvertToModel {
	return &ConvertToModel{
		language:         language,
		fallbackLanguage: defaultFallbackLanguage,
	}
}
func (c *ConvertToModel) Graph(nodes []db.Node, edges []db.Edge) *model.Graph {
	g := model.Graph{}
	for _, v := range nodes {
		if len(v.Description) == 0 {
			continue
		}
		description, ok := v.Description[c.language]
		if !ok {
			description, ok = v.Description[c.fallbackLanguage]
			if !ok {
				for firstExistingLanguage := range v.Description {
					description = v.Description[firstExistingLanguage]
					break
				}
			}
		}
		g.Nodes = append(g.Nodes, &model.Node{
			ID:          v.Key,
			Description: description,
		})
	}
	for _, e := range edges {
		if !strings.Contains(e.From, nodePrefix) || !strings.Contains(e.To, nodePrefix) {
			continue
		}
		g.Edges = append(g.Edges, &model.Edge{
			ID:     e.Key,
			From:   strings.Split(e.From, nodePrefix)[1],
			To:     strings.Split(e.To, nodePrefix)[1],
			Weight: e.Weight,
		})
	}
	return &g
}

var nodePrefix = fmt.Sprintf("%s/", COLLECTION_NODES)

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

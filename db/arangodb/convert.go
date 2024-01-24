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

func (c *ConvertToModel) Node(node db.Node) *model.Node {
	if len(node.Description) == 0 {
		return nil
	}
	description, ok := node.Description[c.language]
	if !ok {
		description, ok = node.Description[c.fallbackLanguage]
		if !ok {
			for firstExistingLanguage := range node.Description {
				description = node.Description[firstExistingLanguage]
				break
			}
		}
	}
	res := model.Node{
		ID:          node.Key,
		Description: description,
	}
	resources, ok := node.Resources[c.language]
	if !ok {
		resources, ok = node.Resources[c.fallbackLanguage]
		if !ok {
			for firstExistingLanguage := range node.Resources {
				resources = node.Resources[firstExistingLanguage]
				break
			}
		}
	}
	if ok {
		res.Resources = &resources
	}
	return &res
}

func (c *ConvertToModel) Graph(nodes []db.Node, edges []db.Edge) *model.Graph {
	g := model.Graph{}
	for _, n := range nodes {
		node := c.Node(n)
		if node == nil {
			continue
		}
		g.Nodes = append(g.Nodes, node)
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

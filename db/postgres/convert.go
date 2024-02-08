package postgres

import (
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

func (c *ConvertToModel) Node(node Node) *model.Node {
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
		ID:          itoa(node.ID),
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

func (c *ConvertToModel) Graph(nodes []Node, edges []Edge) *model.Graph {
	g := model.Graph{}
	for _, n := range nodes {
		node := c.Node(n)
		if node == nil {
			continue
		}
		g.Nodes = append(g.Nodes, node)
	}
	for _, e := range edges {
		g.Edges = append(g.Edges, &model.Edge{
			ID:     itoa(e.ID),
			From:   itoa(e.FromID),
			To:     itoa(e.ToID),
			Weight: e.Weight,
		})
	}
	return &g
}

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

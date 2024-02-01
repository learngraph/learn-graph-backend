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

func (c *ConvertToModel) getTranslationOrFallback(text db.Text) (string, bool) {
	returnText, ok := text[c.language]
	if !ok {
		returnText, ok = text[c.fallbackLanguage]
		returnText = LanguageToLanguageFlag[c.fallbackLanguage] + " " + returnText
		if !ok {
			for firstExistingLanguage := range text {
				returnText = text[firstExistingLanguage]
				returnText = LanguageToLanguageFlag[firstExistingLanguage] + " " + returnText
				break
			}
		}
	}
	return returnText, ok
}

func (c *ConvertToModel) Node(node db.Node) *model.Node {
	if len(node.Description) == 0 {
		return nil
	}
	description, _ := c.getTranslationOrFallback(node.Description)
	res := model.Node{
		ID:          node.Key,
		Description: description,
	}
	resources, ok := c.getTranslationOrFallback(node.Resources)
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

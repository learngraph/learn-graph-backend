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

var LanguageToLanguageFlag = map[string]string{
	"en": "🇺🇸",
	"de": "🇩🇪",
	"zh": "🇹🇼",
	"es": "🇪🇸",
	"fr": "🇫🇷",
	"it": "🇮🇹",
	"ja": "🇯🇵",
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

func (c *ConvertToModel) Node(node Node) *model.Node {
	if len(node.Description) == 0 {
		return nil
	}
	description, _ := c.getTranslationOrFallback(node.Description)
	res := model.Node{
		ID:          itoa(node.ID),
		Description: description,
	}
	resources, ok := c.getTranslationOrFallback(node.Resources)
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

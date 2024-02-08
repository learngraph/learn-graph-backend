package db

import "github.com/suxatcode/learn-graph-poc-backend/graph/model"

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

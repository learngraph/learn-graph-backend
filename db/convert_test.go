package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/suxatcode/learn-graph-poc-backend/graph/model"
)

func TestConvertToDBText(t *testing.T) {
	for _, test := range []struct {
		Name string
		Inp  *model.Text
		Exp  Text
	}{
		{
			Name: "empty translations should return empty map (not nil)",
			Inp: &model.Text{
				Translations: []*model.Translation{},
			},
			Exp: Text{},
		},
		{
			Name: "non-empty, but nil translations should return empty map",
			Inp: &model.Text{
				Translations: []*model.Translation{nil, nil},
			},
			Exp: Text{},
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
			Exp: Text{
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

// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package model

type Edge struct {
	ID   string `json:"id"`
	From string `json:"from"`
	To   string `json:"to"`
}

type Error struct {
	Message *string `json:"Message"`
}

type Graph struct {
	Nodes []*Node `json:"nodes"`
	Edges []*Edge `json:"edges"`
}

type Node struct {
	ID string `json:"id"`
}

type Text struct {
	Translations []*Translation `json:"translations"`
}

type Translation struct {
	Language *string `json:"language"`
	Content  *string `json:"content"`
}

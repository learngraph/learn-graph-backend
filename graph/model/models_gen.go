// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package model

type CreateEntityResult struct {
	ID     string  `json:"ID"`
	Status *Status `json:"Status"`
}

type CreateUserResult struct {
	Login *LoginResult `json:"login"`
}

type Edge struct {
	ID     string  `json:"id"`
	From   string  `json:"from"`
	To     string  `json:"to"`
	Weight float64 `json:"weight"`
}

type Graph struct {
	Nodes []*Node `json:"nodes"`
	Edges []*Edge `json:"edges"`
}

type LoginAuthentication struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResult struct {
	Success  bool    `json:"success"`
	Token    string  `json:"token"`
	UserID   string  `json:"userID"`
	UserName string  `json:"userName"`
	Message  *string `json:"message"`
}

type Node struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type Status struct {
	Message string `json:"Message"`
}

type Text struct {
	Translations []*Translation `json:"translations"`
}

type Translation struct {
	Language string `json:"language"`
	Content  string `json:"content"`
}

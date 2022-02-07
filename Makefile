build:
	CGO_ENABLED=0 go build -o main

gqlgen:
	go run github.com/99designs/gqlgen generate

build:
	CGO_ENABLED=0 go build -o main

dev:
	go run main.go

gqlgen:
	go run github.com/99designs/gqlgen generate --config ./graph/gqlgen.yml

.PHONY: build gqlgen dev

build:
	CGO_ENABLED=0 go build -o main

dev:
	go run main.go

gqlgen:
	go run github.com/99designs/gqlgen generate --config ./graph/gqlgen.yml

mockgen:
	rm $$(find -name '*_mock.go')
	go generate ./...

mockgen-install:
	go install github.com/golang/mock/mockgen@v1.6.0

test:
	go test ./...

test-integration:
	go test ./... -tags integration


.PHONY: build gqlgen dev

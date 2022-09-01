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
	# must be executed separately as both access the test db
	go test -tags integration ./db/...
	go test -tags integration ./internal/app/...

.PHONY: build gqlgen dev mockgen mockgen-install test test-integration

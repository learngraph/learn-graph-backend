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
	#docker-compose -f docker-compose-test.yml up -d
	make test-integration-no-docker
	#docker-compose -f docker-compose-test.yml down

test-integration-no-docker:
	go test ./... -tags integration


.PHONY: build gqlgen dev mockgen mockgen-install test test-integration test-integration-no-docker

build:
	CGO_ENABLED=0 go build -o main
.PHONY: build

dev-pre-created-token:
	DB_ARANGO_JWT_TOKEN=eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpYXQiOjE2NjIzNzUzMDgsImlzcyI6ImFyYW5nb2RiIiwic2VydmVyX2lkIjoibGVhcm5ncmFwaC1iYWNrZW5kLXRlc3QifQ.MSbe9s_Hv7lAdo4DeitpYrtZ5ieRUK6nrHnq0i57aGg \
		go run main.go
.PHONY: dev-pre-created-token

dev-auth-test:
	DB_ARANGO_JWT_SECRET_PATH=./test/data/jwtSecret \
		go run main.go
.PHONY: dev-auth-test

dev-auth-prod:
	DB_ARANGO_JWT_SECRET_PATH=./docker-data/arangodb_secrets/jwtSecret \
		go run main.go
.PHONY: dev-auth-prod

gqlgen:
	go run github.com/99designs/gqlgen generate --config ./graph/gqlgen.yml
.PHONY: gqlgen

mockgen:
	rm -f $$(find -name '*_mock.go' -not -path "./docker-data/*" )
	go generate ./...
.PHONY: mockgen

mockgen-install:
	go install github.com/golang/mock/mockgen@v1.6.0
.PHONY: mockgen-install

test:
	go test ./...
.PHONY: test

test-integration:
	# must be executed separately as both re-create the test db
	go test -tags integration ./db/...
	go test -tags integration ./internal/app/...
.PHONY: test-integration

fmt:
	go fmt ./...
.PHONY: fmt

# only for testing, should be integrated into VCS (i.e. github push to main)
build-and-push:
	docker build . -t xsbzgtoi/learn-graph-backend:latest
	docker push xsbzgtoi/learn-graph-backend:latest
.PHONY: build-and-push

build:
	CGO_ENABLED=0 go build -o main
.PHONY: build

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
	@# must be executed separately as both re-create the test db
	go test -tags integration ./db/...
	go test -tags integration ./internal/app/...
.PHONY: test-integration

test-watch:
	while true; do inotifywait -e modify,close_write,move,delete --include='.*\.go' -r ./; make test && make test-integration; done
.PHONY: test-watch

fmt:
	go fmt ./...
.PHONY: fmt

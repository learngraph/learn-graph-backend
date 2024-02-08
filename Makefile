build:
	CGO_ENABLED=0 go build -o main
.PHONY: build

build-run-continuous:
	docker-compose -f ./docker-compose/wgo.yml up
.PHONY: build-run-continuous

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

dev-tools-install:
	go install github.com/golang/mock/mockgen@v1.6.0
.PHONY: dev-tools-install

test:
	go test ./...
.PHONY: test

test-integration:
	@# must be executed separately as both re-create the test db
	go test -tags integration ./db/arangodb/...
	go test -tags integration ./db/postgres/...
	go test -tags integration ./internal/app/...
.PHONY: test-integration

test-watch:
	while true; do inotifywait -e modify,close_write,move,delete --include='.*\.go' -r ./; make test && make test-integration; done
.PHONY: test-watch

fmt:
	go fmt ./...
.PHONY: fmt

db-password:
	cat /dev/random | head -c 30 | base64 | head -c-1 > ./docker-data/postgres.pw
.PHONY: db-password

## Usage

### Environment Variables
```
PORT                        - port to listen on for graphql queries (& graphql playground), default: 8080
PRODUCTION                  - true/false, enables/disables production mode: changes logging output, disabled GraphQL playground, etc.
LOG_LEVEL                   - Levels are {trace, debug, info, warn, error, fatal, panic}. See github.com/rs/zerolog@v1.19.0/log.go for possible values.
TIMEOUT                     - HTTP timeouts (read and write) as Golang time string, e.g. "30s" for 30 seconds.
DB_ARANGO_HOST              - arango db <protocol>://<host>[:<port>], default: http://localhost:8529
DB_ARANGO_JWT_TOKEN         - arango db signed JWT token, takes precedence over DB_ARANGO_JWT_SECRET_PATH
DB_ARANGO_JWT_SECRET_PATH   - arango db JWT secret (that can sign new tokens)
DB_ARANGO_NO_AUTH           - disabled arango db authentication (only works if db is also started with NO_AUTH option)
```
See `grep -r 'env:' .`.

## Development

### Testing
Run unittests via make
```sh
make test
```

To enable `gopls` for the `*_integration_test.go`-files use
```sh
export GOFLAGS='-tags=integration'
```

Integration tests require a testing database with no authentication
```sh
docker-compose -f docker-compose-test.yml up -d
# wait for startup to complete
make test-integration
```

New integration tests should be a inside a file ending in
`_integration_test.go` and contain as first line the tag `integration`.
```go
//go:build integration
```

#### Writing New Tests
We use the 'assert' package.

We use gomock, to install it execute
```sh
make mockgen-install
```

To regenerate mocks execute
```sh
make mockgen
```

#### Testing by Hand
Sample query:
```sh
curl http://localhost:8124/query -H 'Content-Type: application/json' -d '{"operationName":"getGraph","variables":{},"query":"query getGraph {\n  graph {\n    nodes {\n      id\n      __typename\n    }\n    edges {\n      id\n      from\n      to\n      __typename\n    }\n    __typename\n  }\n}"}';echo
```

## Technologies

### Graph Database: ArangoDB
We use [ArangoDB](https://github.com/arangodb/arangodb) as database and it's
official [go-driver](https://github.com/arangodb/go-driver).

Usefull resources for JWT tokens: [jwt.io](https://jwt.io/) &
[jwtgen](https://www.npmjs.com/package/jwtgen).

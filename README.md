# Learngraph Backend

See

- [learngraph.org](https://learngraph.org/),
- [about us](https://learngraph.org/about).

## Contributing

### How to Contribute?

- Commit messages should follow the [conventional commits guideline](https://www.conventionalcommits.org/en/v1.0.0/),
- Create a PR & wait for review,
- PRs should be "squashed & merged"

### Running the Application
```sh
docker-compose up
# or for continuous builds
docker-compose -f docker-compose/wgo.yml up
```

#### Environment Variables
```
PORT                        - port to listen on for graphql queries (& graphql playground), default: 8080
PRODUCTION                  - true/false, enables/disables production mode: changes logging output, disabled GraphQL playground, etc.
LOG_LEVEL                   - Levels are {trace, debug, info, warn, error, fatal, panic}. See github.com/rs/zerolog@v1.19.0/log.go for possible values.
TIMEOUT                     - HTTP timeouts (read and write) as Golang time string, e.g. "30s" for 30 seconds.
DB_POSTGRES_HOST            - postgresql db host, e.g. (default: "localhost")
DB_POSTGRES_PASSWORD        - postgresql db password for authentication (default: "example")
```
See `grep -r 'env:' .`.

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
docker-compose -f docker-compose/test.yml up -d
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
make dev-tools-install
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

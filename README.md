## Usage

### Environment Variables
```
PORT                        - port to listen on for graphql queries (& graphql playground), default: 8080
DB_ARANGO_HOST              - arango db <protocol>://<host>[:<port>], default: http://localhost:8529
DB_ARANGO_JWT_TOKEN         - arango db signed JWT token, takes precedence over DB_ARANGO_JWT_SECRET_PATH
DB_ARANGO_JWT_SECRET_PATH   - arango db JWT secret (that can sign new tokens)
DB_ARANGO_NO_AUTH           - disabled arango db authentication (only works if db is also started with NO_AUTH option)
```
See `grep -r 'env:'`.

## Development

### Testing
We use gomock, to install execute

```sh
make mockgen-install
```

To regenerate mocks execute

```sh
make mockgen
```

Sample query:
```sh
curl http://localhost:8124/query -H 'Content-Type: application/json' -d '{"operationName":"getGraph","variables":{},"query":"query getGraph {\n  graph {\n    nodes {\n      id\n      __typename\n    }\n    edges {\n      id\n      from\n      to\n      __typename\n    }\n    __typename\n  }\n}"}';echo
```

Usefull for jwt tokens: [jwt.io](https://jwt.io/) & [jwtgen](https://www.npmjs.com/package/jwtgen).

### Graph Database: ArangoDB
We use [ArangoDB](https://github.com/arangodb/arangodb) as database and it's
official [go-driver](https://github.com/arangodb/go-driver).

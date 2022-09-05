## Usage

### Environment Variables
```
PORT                - port to listen on for graphql queries, default: 8080
DB_ARANGO_HOST      - arango db <protocol>://<host>[:<port>], default: -
DB_ARANGO_USER      - arango db login user
DB_ARANGO_PASSWORD  - arango db password
```

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

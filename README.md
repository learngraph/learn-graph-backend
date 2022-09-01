
## Graph Database: ArangoDB

## Testing
We use gomock, to install execute

```sh
make mockgen-install
```

To regenerate mocks execute

```sh
make mockgen
```

## TMP
Sample query:
```sh
curl http://localhost:8124/query -H 'Content-Type: application/json' -d '{"operationName":"getGraph","variables":{},"query":"query getGraph {\n  graph {\n    nodes {\n      id\n      __typename\n    }\n    edges {\n      id\n      from\n      to\n      __typename\n    }\n    __typename\n  }\n}"}';echo
```

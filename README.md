## Testing
dgraph GUI:
```sh
docker run -p "8000:8000" dgraph/ratel
```

test API requests:
```sh
curl -H "Content-Type: application/json" -d '{"query":"schema {}"}' 'http://localhost:8080/query?timeout=20s'
curl -H "Content-Type: application/json" -d '{"query":"schema {}"}' 'http://localhost:8080/query?timeout=20s'|python -m json.tool
```

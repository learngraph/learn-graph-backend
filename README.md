## Testing
```sh
curl -H "Content-Type: application/json" -d '{"query":"schema {}"}' 'http://localhost:8080/query?timeout=20s'
curl -H "Content-Type: application/json" -d '{"query":"schema {}"}' 'http://localhost:8080/query?timeout=20s'|python -m json.tool
```

#!/bin/bash
docker container exec -it learn-graph-backend-backend-1 bash -c "go run /src/cmd/stress-test/create-db.go" 

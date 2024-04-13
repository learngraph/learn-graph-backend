#!/bin/bash
echo "go run /src/cmd/stress-test/create-db.go $@" | docker container exec -i learn-graph-backend-backend-1 bash

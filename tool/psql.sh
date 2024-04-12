#!/bin/bash
docker container exec -it learn-graph-backend-postgres-1 bash -c 'su postgres -c "psql -U learngraph"'

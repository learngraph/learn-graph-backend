package app

import (
	"log"
	"net/http"
	"os"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph"
	"github.com/suxatcode/learn-graph-poc-backend/graph/generated"
)

const defaultPort = "8080"

func graphHandler() http.Handler {
	conf := db.GetEnvConfig()
	log.Printf("Query(): config: %#v", conf)
	db, err := db.NewArangoDB(conf)
	if err != nil {
		log.Fatalf("failed to connect to DB: %v", err)
	}
	return handler.NewDefaultServer(
		generated.NewExecutableSchema(generated.Config{Resolvers: &graph.Resolver{Db: db}}),
	)
}

func addMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			log.Printf("r=%v, headers=%v", r.RemoteAddr, r.Header)
			next.ServeHTTP(w, r)
		},
	)
}

func runGQLServer() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", addMiddleware(graphHandler()))

	// TODO: timeouts for incomming connections
	log.Printf("connect to http://0.0.0.0:%s/ for GraphQL playground", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

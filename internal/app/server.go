package app

import (
	"net/http"
	"os"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/graph"
	"github.com/suxatcode/learn-graph-poc-backend/graph/generated"
	"github.com/suxatcode/learn-graph-poc-backend/middleware"
)

const defaultPort = "8080"

func graphHandler() http.Handler {
	conf := db.GetEnvConfig()
	log.Info().Msgf("Config: %#v", conf)
	db, err := db.NewArangoDB(conf)
	if err != nil {
		log.Fatal().Msgf("failed to connect to DB: %v", err)
	}
	return handler.NewDefaultServer(
		generated.NewExecutableSchema(generated.Config{Resolvers: &graph.Resolver{Db: db}}),
	)
}

func runGQLServer() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	// TODO: make it env-configurable
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", middleware.AddHttp(graphHandler()))

	// TODO: timeouts for incomming connections
	log.Info().Msgf("connect to http://0.0.0.0:%s/ for GraphQL playground", port)
	log.Fatal().Msgf("ListenAndServe: %s", http.ListenAndServe(":"+port, nil))
}

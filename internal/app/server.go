package app

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/caarlos0/env/v6"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/suxatcode/learn-graph-poc-backend/db"
	"github.com/suxatcode/learn-graph-poc-backend/db/postgres"
	"github.com/suxatcode/learn-graph-poc-backend/graph"
	"github.com/suxatcode/learn-graph-poc-backend/graph/generated"
	"github.com/suxatcode/learn-graph-poc-backend/internal/controller"
	"github.com/suxatcode/learn-graph-poc-backend/middleware"
)

const defaultPort = "8080"

type Config struct {
	Production bool `env:"PRODUCTION" envDefault:"false"`
	// Levels are {trace, debug, info, warn, error, fatal, panic}.
	// See github.com/rs/zerolog@v1.19.0/log.go for possible values.
	LogLevel string `env:"LOGLEVEL" envDefault:"debug"`
	// HTTP timeouts (read and write)
	HTTPTimeout time.Duration `env:"TIMEOUT" envDefault:"5s"`
}

func GetEnvConfig() Config {
	conf := Config{}
	env.Parse(&conf)
	return conf
}

func RetryAtIntervals(fn func() error, intervals []time.Duration) {
	var err error
	err = fn()
	i := 0
	for err != nil {
		time.Sleep(intervals[i])
		if i < len(intervals)-1 {
			i++
		}
		err = fn()
	}
}

func graphHandler(conf db.Config) (http.Handler, db.DB) {
	var (
		backend db.DB
		err     error
	)
	RetryAtIntervals(func() error {
		backend, err = postgres.NewPostgresDB(conf)
		if err != nil {
			log.Error().Msgf("failed to connect to DB: %v", err)
		}
		return err
	}, []time.Duration{
		1 * time.Second,
		5 * time.Second,
		5 * time.Second,
		10 * time.Second,
	})
	ctrl := controller.NewController(backend, controller.NewForceSimulationLayouter())
	go ctrl.PeriodicGraphEmbeddingComputation(context.Background())
	return middleware.AddAll(handler.NewDefaultServer(
		generated.NewExecutableSchema(generated.Config{Resolvers: &graph.Resolver{
			Db:   backend, /*TODO(skep): to be removed once all calls go through controller*/
			Ctrl: ctrl,
		}}),
	)), backend
}

func runGQLServer() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	conf := GetEnvConfig()
	level, err := zerolog.ParseLevel(conf.LogLevel)
	if err != nil {
		println("failed to parse LogLevel: '" + conf.LogLevel + "', setting to debug")
		level = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(level)
	if !conf.Production {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}

	handler := http.NewServeMux()
	if !conf.Production {
		handler.Handle("/", playground.Handler("GraphQL playground", "/query"))
	}
	dbconf := db.GetEnvConfig()
	log.Info().Msgf("Config: %#v", dbconf)
	graphQLhandler, _ := graphHandler(dbconf)
	handler.Handle("/query", graphQLhandler)
	server := http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  conf.HTTPTimeout,
		WriteTimeout: conf.HTTPTimeout,
	}
	log.Info().Msgf("connect to http://0.0.0.0:%s/ for GraphQL playground", port)
	log.Fatal().Msgf("ListenAndServe: %s", server.ListenAndServe())
}

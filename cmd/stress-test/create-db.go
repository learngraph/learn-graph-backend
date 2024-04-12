package main

import (
	"fmt"
	"log"

	dbiface "github.com/suxatcode/learn-graph-poc-backend/db"
	dbimpl "github.com/suxatcode/learn-graph-poc-backend/db/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	conf := dbiface.Config{
		PGHost: "postgres",
		PGPassword: "example",
	}
	pgConfig := postgres.Config{
		DSN: fmt.Sprintf("host=%s user=learngraph password=%s dbname=learngraph port=5432 sslmode=disable", conf.PGHost, conf.PGPassword),
	}
	db, err := gorm.Open(postgres.New(pgConfig), &gorm.Config{})
	if err != nil {
		log.Fatalf("%v: authentication with DSN: '%v' failed", err, pgConfig.DSN)
	}
	nodes := []dbimpl.Node{}
	if err := db.Find(&nodes).Error; err != nil {
		log.Fatal(err)
	}
	log.Print(nodes)
}

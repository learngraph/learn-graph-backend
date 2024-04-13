package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"

	dbiface "github.com/suxatcode/learn-graph-poc-backend/db"
	dbimpl "github.com/suxatcode/learn-graph-poc-backend/db/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	FirstID = flag.Int("first_id", 0, "first db-id to use for generated nodes")
	NNodes  = flag.Int("nodes", 0, "number of nodes to generate")
	NEdges  = flag.Int("edges", 0, "number of edges to generate")
)

func init() {
	flag.Parse()
}

func main() {
	conf := dbiface.Config{
		PGHost:     "postgres",
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
	ID := uint(*FirstID)
	for i := 0; i < *NNodes; i++ {
		nodes = append(nodes, dbimpl.Node{Model: gorm.Model{ID: ID}, Description: dbiface.Text{"en": fmt.Sprintf("n%d", i)}})
		ID += 1
	}
	if len(nodes) > 0 {
		if err := db.Create(&nodes).Error; err != nil {
			log.Fatal(err)
		}
		log.Printf("Created %d nodes", len(nodes))
	}
	edges := []dbimpl.Edge{}
	randomNode := func() uint {
		return nodes[rand.Int()%len(nodes)].ID
	}
	exists := func(from, to uint) bool {
		for _, edge := range edges {
			if edge.FromID == from && edge.ToID == to {
				return true
			}
		}
		return false
	}
	for i := 0; i < *NEdges; i++ {
		from, to := randomNode(), randomNode()
		for {
			if !exists(from, to) {
				break
			}
			from, to = randomNode(), randomNode()
		}
		edges = append(edges, dbimpl.Edge{FromID: from, ToID: to, Weight: 0.1 + rand.Float64()*9.9})
	}
	if len(edges) > 0 {
		if err := db.Create(&edges).Error; err != nil {
			log.Fatal(err)
		}
		log.Printf("Created %d edges", len(edges))
	}
	n_nodes := int64(0)
	if err := db.Find(&dbimpl.Node{}).Count(&n_nodes).Error; err != nil {
		log.Fatal(err)
	}
	n_edges := int64(0)
	if err := db.Find(&dbimpl.Edge{}).Count(&n_edges).Error; err != nil {
		log.Fatal(err)
	}
	log.Printf("The Learngraph now has %d nodes and %d edges!", n_nodes, n_edges)
}

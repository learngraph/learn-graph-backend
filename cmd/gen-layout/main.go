/*
 * gen-layout runs a force-simulation on the graph received on stdin in json
 * format
 */
package main

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"os"

	"github.com/quartercastle/vector"
	"github.com/suxatcode/learn-graph-poc-backend/layout"
)

func main() {
	graph := layout.Graph{}
	err := json.NewDecoder(os.Stdin).Decode(&graph)
	if err != nil {
		log.Fatal(err)
	}
	fs := layout.NewForceSimulation(layout.DefaultForceSimulationConfig)
	_, stats := fs.ComputeLayout(context.Background(), graph.Nodes, graph.Edges)
	log.Print("Nodes:")
	for i, node := range graph.Nodes {
		if math.IsNaN(node.Pos.X()) && math.IsNaN(node.Pos.Y()) {
			graph.Nodes[i].Pos = vector.Vector{1.0, 1.0}
		}
		log.Printf("%#v", *node)
	}
	log.Printf("Stats: %#v", stats)
	err = json.NewEncoder(os.Stdout).Encode(&graph)
	if err != nil {
		log.Fatal(err)
	}
}

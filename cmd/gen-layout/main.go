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
	"runtime"

	"github.com/suxatcode/learn-graph-poc-backend/layout"
)

func main() {
	graph := layout.Graph{}
	err := json.NewDecoder(os.Stdin).Decode(&graph)
	if err != nil {
		log.Fatal(err)
	}
	max_y := 10000.0
	conf := layout.ForceSimulationConfig{
		AlphaInit:           1.0,
		AlphaDecay:          0.005, // normal: 0.05
		AlphaTarget:         0.1,
		Parallelization:     runtime.NumCPU(),
		Gravity:             true,
		GravityStrength:     0.5,
		RepulsionMultiplier: 100000.0,
		Rect:                layout.Rect{X: 0.0, Y: 0.0, Width: max_y * 2, Height: max_y}, ScreenMultiplierToClampPosition: 1000,
	}
	log.Printf("Config{%#v}", conf)
	fs := layout.NewForceSimulation(conf)
	_, stats := fs.ComputeLayout(context.Background(), graph.Nodes, graph.Edges)
	for i, node := range graph.Nodes {
		if math.IsNaN(node.Pos.X()) && math.IsNaN(node.Pos.Y()) {
			graph.Nodes[i].Pos = conf.RandomVectorInside()
		}
		//log.Printf("%#v", *node)
	}
	log.Printf("Layout took %d ms to compute and used %d iterations.", stats.TotalTime.Milliseconds(), stats.Iterations)
	err = json.NewEncoder(os.Stdout).Encode(&graph)
	if err != nil {
		log.Fatal(err)
	}
}

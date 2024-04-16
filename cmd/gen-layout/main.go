// adapted from https://github.com/jwhandley/graphyz/blob/main/main.go
package main

import (
	"os"
	"runtime/pprof"
	"sync"
	"time"
)

// values taken from https://github.com/jwhandley/graphyz/blob/main/config.yaml
var config = struct {
	ScreenWidth, ScreenHeight             float64
	VelocityDecay, GravityStrength, Theta float64
	AlphaTarget, AlphaDecay, AlphaInit    float64
	Gravity, BarnesHut, Debug             bool
	Capacity                              int
}{
	ScreenWidth:     1200,
	ScreenHeight:    800,
	Gravity:         false,
	GravityStrength: 0.5,
	BarnesHut:       true,
	VelocityDecay:   0.1,
	Capacity:        1000,
	Theta:           0.75,
	Debug:           true,
	AlphaTarget:     0.1,
	AlphaDecay:      0.05,
	AlphaInit:       1.0,
}

var temperature float64
var mutex sync.Mutex

const EPSILON = 1e-2

func updatePhysics(graph *Graph) {
	var frameTime float64 = 0.016
	rect := Rect{-float64(config.ScreenWidth), -float64(config.ScreenHeight), 2 * float64(config.ScreenWidth), 2 * float64(config.ScreenHeight)}
	qt := NewQuadTree(rect)
	for {
		startTime := time.Now()
		graph.ApplyForce(frameTime, qt)
		frameTime = float64(time.Since(startTime).Seconds())
		temperature += (config.AlphaTarget - temperature) * config.AlphaDecay * frameTime
		// TODO: exit cond. related to temperature
	}
}

func main() {
	if config.Debug {
		f, err := os.Create("cpu.pp")
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	temperature = config.AlphaInit
	graph := Graph{
		Nodes: []*Node{},
		Edges: []*Edge{},
	}
	updatePhysics(&graph)
}

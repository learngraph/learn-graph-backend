// adapted from https://github.com/jwhandley/graphyz/blob/main/main.go
package layout

import (
	"math"
	"os"
	"runtime/pprof"
	"time"

	"github.com/quartercastle/vector"
)

// values taken from https://github.com/jwhandley/graphyz/blob/main/config.yaml
var config = struct {
	ScreenWidth, ScreenHeight             float64
	VelocityDecay, GravityStrength, Theta float64
	AlphaTarget, AlphaDecay, AlphaInit    float64
	Gravity, BarnesHut, Debug             bool
	Capacity                              int
	Epsilon                               float64
}{
	ScreenWidth:     1200,
	ScreenHeight:    800,
	Gravity:         false,
	GravityStrength: 0.5,
	BarnesHut:       true,
	VelocityDecay:   0.1,
	Capacity:        10,
	Theta:           0.75,
	Debug:           true,
	AlphaTarget:     0.1,
	AlphaDecay:      0.05,
	AlphaInit:       1.0,
	Epsilon:         1e-2,
}

func NewForceSimulation(conf ForceSimulationConfig) *ForceSimulation {
	if conf.DefaultNodeRadius == 0.0 {
		conf.DefaultNodeRadius = DefaultForceSimulationConfig.DefaultNodeRadius
	}
	if conf.Rect.Width == 0.0 || conf.Rect.Height == 0.0 {
		conf.Rect = DefaultForceSimulationConfig.Rect
	}
	return &ForceSimulation{conf: conf}
}

var DefaultForceSimulationConfig = ForceSimulationConfig{
	Rect:    Rect{0.0, 0.0, config.ScreenWidth, config.ScreenHeight},
	EPSILON: config.Epsilon,
	DefaultNodeRadius: 1.0,
}

type ForceSimulationConfig struct {
	Rect    Rect
	DefaultNodeRadius float64
	EPSILON float64
}

type ForceSimulation struct {
	conf        ForceSimulationConfig
	temperature float64
}

func (fs *ForceSimulation) ComputeLayout(nodes []*Node, edges []*Edge) []*Node {
	if config.Debug {
		f, err := os.Create("cpu.pp")
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	graph := NewGraph(nodes, edges, fs)
	frameTime := float64(0.016)
	qt := NewQuadTree(&QUADTREE_DEFAULT_CONFIG, fs, fs.conf.Rect)
	fs.temperature = config.AlphaInit
	for {
		startTime := time.Now()
		graph.ApplyForce(frameTime, qt)
		frameTime = float64(time.Since(startTime).Seconds())
		fs.temperature += (config.AlphaTarget - fs.temperature) * config.AlphaDecay * frameTime
		break // TODO: exit cond. related to temperature
	}
	return graph.Nodes
}

func (fs *ForceSimulation) calculateRepulsionForce(b1 Body, b2 Body) vector.Vector {
	delta := b1.position().Sub(b2.position())
	dist := delta.Magnitude()
	if dist*dist < b1.size()*b2.size() {
		dist = b1.size() * b2.size()
	}
	scale := b1.size() * b2.size() * fs.temperature
	force := delta.Unit().Scale(10 * scale / dist)
	return force
}

func (fs *ForceSimulation) calculateAttractionForce(from *Node, to *Node, weight float64) vector.Vector {
	delta := from.pos.Sub(to.pos)
	dist := delta.Magnitude()
	if dist < fs.conf.EPSILON {
		dist = fs.conf.EPSILON
	}
	s := float64(math.Min(float64(from.radius), float64(to.radius)))
	l := float64(from.radius + to.radius)
	force := delta.Unit().Scale((dist - l) / s * weight * fs.temperature)
	return force
}

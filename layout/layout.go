// adapted from https://github.com/jwhandley/graphyz/blob/main/main.go
package layout

import (
	"math"
	"math/rand"
	"os"
	"runtime/pprof"
	"time"

	"github.com/quartercastle/vector"
)

// values taken from https://github.com/jwhandley/graphyz/blob/main/config.yaml
var config = struct {
	ScreenWidth, ScreenHeight             float64
	VelocityDecay, GravityStrength, Theta float64
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
	Epsilon:         1e-2,
}

func NewForceSimulation(conf ForceSimulationConfig) *ForceSimulation {
	if conf.Rect.Width == 0.0 || conf.Rect.Height == 0.0 {
		conf.Rect = DefaultForceSimulationConfig.Rect
	}
	if conf.DefaultNodeRadius == 0.0 {
		conf.DefaultNodeRadius = DefaultForceSimulationConfig.DefaultNodeRadius
	}
	if conf.MinDistanceBeweenNodes == 0.0 {
		conf.MinDistanceBeweenNodes = DefaultForceSimulationConfig.MinDistanceBeweenNodes
	}
	if conf.AlphaInit == 0.0 {
		conf.AlphaInit = DefaultForceSimulationConfig.AlphaInit
	}
	if conf.AlphaDecay == 0.0 {
		conf.AlphaDecay = DefaultForceSimulationConfig.AlphaDecay
	}
	if conf.AlphaTarget == 0.0 {
		conf.AlphaTarget = DefaultForceSimulationConfig.AlphaTarget
	}
	if conf.FrameTime == 0.0 {
		conf.FrameTime = DefaultForceSimulationConfig.FrameTime
	}
	if conf.RandomFloat == nil {
		conf.RandomFloat = func() float64 { return rand.Float64() }
	}
	return &ForceSimulation{conf: conf}
}

var DefaultForceSimulationConfig = ForceSimulationConfig{
	Rect:                   Rect{0.0, 0.0, config.ScreenWidth, config.ScreenHeight},
	MinDistanceBeweenNodes: config.Epsilon,
	DefaultNodeRadius:      1.0,
	AlphaInit:              1.0,
	AlphaDecay:             0.05,
	AlphaTarget:            0.1,
	FrameTime:              0.016,
}

type ForceSimulationConfig struct {
	Rect                   Rect
	DefaultNodeRadius      float64
	MinDistanceBeweenNodes float64
	// initial temperature of simulation
	AlphaInit float64
	// decay of temperature per tick
	AlphaDecay float64
	// target temperature of simulation
	AlphaTarget float64
	// FrameTime describes the time passed per tick of simulation.
	// Increasing this value increases the range of the position updates per
	// tick, and thus decreases the precision of the simulation.
	//	=> too high FrameTime might lead to over-estimating optimal positions
	//	   and thus never reaching equilibrium
	//	=> too low FrameTime might lead to a lot of computation without ever
	//	   reaching the optimal position
	FrameTime   float64
	RandomFloat func() float64
}

type ForceSimulation struct {
	conf        ForceSimulationConfig
	temperature float64
}

type Stats struct {
	Iterations uint64
	TotalTime  time.Duration
}

func (fs *ForceSimulation) ComputeLayout(nodes []*Node, edges []*Edge) ([]*Node, Stats) {
	if config.Debug {
		f, err := os.Create("cpu.pp")
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	graph := NewGraph(nodes, edges, fs)
	qt := NewQuadTree(&QUADTREE_DEFAULT_CONFIG, fs, fs.conf.Rect)
	fs.temperature = fs.conf.AlphaInit
	startTime := time.Now()
	stats := Stats{}
	for {
		graph.ApplyForce(fs.conf.FrameTime, qt)
		stats.Iterations += 1
		freezeOverTime := fs.conf.AlphaDecay
		fs.temperature += (fs.conf.AlphaTarget - fs.temperature) * freezeOverTime
		if isClose(fs.conf.AlphaTarget, fs.temperature) {
			stats.TotalTime = time.Since(startTime)
			break
		}
	}
	return graph.Nodes, stats
}
func isClose(a, b float64) bool {
	abs_tol := 1e-5
	diff := math.Abs(a - b)
	if diff <= abs_tol {
		return true
	}
	return false
}

func (fs *ForceSimulation) calculateRepulsionForce(b1 Body, b2 Body) vector.Vector {
	force := b1.position().Sub(b2.position())
	dist := force.Magnitude()
	if dist*dist < b1.size()*b2.size() {
		dist = b1.size() * b2.size()
	}
	scale := b1.size() * b2.size() * fs.temperature
	vector.In(force).Unit().Scale(10 * scale / dist)
	return force
}

func (fs *ForceSimulation) calculateAttractionForce(from *Node, to *Node, weight float64) vector.Vector {
	delta := from.pos.Sub(to.pos)
	dist := clamp(delta.Magnitude(), fs.conf.MinDistanceBeweenNodes, math.Inf(+1))
	s := float64(math.Min(float64(from.radius), float64(to.radius)))
	l := float64(from.radius + to.radius)
	force := delta.Unit().Scale((dist - l) / s * weight * fs.temperature)
	return force
}

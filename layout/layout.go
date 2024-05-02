// adapted from https://github.com/jwhandley/graphyz/blob/main/main.go
package layout

import (
	"context"
	"math"
	"math/rand"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/quartercastle/vector"
)

// values taken from https://github.com/jwhandley/graphyz/blob/main/config.yaml
var config = struct {
	ScreenWidth, ScreenHeight float64
	VelocityDecay             float64
	Theta                     float64
	BarnesHut, Debug          bool
	Capacity                  int
	Epsilon                   float64
}{
	ScreenWidth:   1200,
	ScreenHeight:  800,
	BarnesHut:     true,
	VelocityDecay: 0.1,
	Capacity:      10,
	Theta:         0.75,
	Debug:         true,
	Epsilon:       1e-2,
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
	if conf.RepulsionMultiplier == 0.0 {
		conf.RepulsionMultiplier = DefaultForceSimulationConfig.RepulsionMultiplier
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
	if conf.ScreenMultiplierToClampPosition == 0.0 {
		conf.ScreenMultiplierToClampPosition = DefaultForceSimulationConfig.ScreenMultiplierToClampPosition
	}
	if conf.GravityStrength == 0.0 {
		conf.GravityStrength = DefaultForceSimulationConfig.GravityStrength
	}
	return &ForceSimulation{conf: conf, temperature: conf.AlphaInit}
}

var DefaultForceSimulationConfig = ForceSimulationConfig{
	Rect:                            Rect{0.0, 0.0, config.ScreenWidth, config.ScreenHeight},
	MinDistanceBeweenNodes:          config.Epsilon,
	DefaultNodeRadius:               1.0,
	RepulsionMultiplier:             10.0,
	AlphaInit:                       1.0,
	AlphaDecay:                      0.05,
	AlphaTarget:                     0.1,
	FrameTime:                       0.016,
	ScreenMultiplierToClampPosition: 10.0,
	Parallelization:                 runtime.NumCPU(),
	Gravity:                         true,
	GravityStrength:                 0.5,
}

type ForceSimulationConfig struct {
	Rect                   Rect
	DefaultNodeRadius      float64
	MinDistanceBeweenNodes float64
	RepulsionMultiplier    float64
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
	FrameTime                       float64
	RandomFloat                     func() float64
	ScreenMultiplierToClampPosition float64
	// Parallelization is the number of goroutines spawned using BarnesHut
	// algorithm *times 4*, i.e. if Parallelization = 1, then for each
	// top-level sub-space (4) a goroutine a spawned.
	Parallelization int
	// Gravity enables a force directed towards the center of the simulation,
	// to keep nodes from flying away to infinity
	Gravity         bool
	GravityStrength float64
}

func randomVectorInside(rect Rect, rndSource func() float64) vector.Vector {
	return vector.Vector{
		rect.X + rndSource()*rect.Width,
		rect.Y + rndSource()*rect.Height,
	}
}

func (fsconf ForceSimulationConfig) RandomVectorInside() vector.Vector {
	if fsconf.RandomFloat == nil {
		fsconf.RandomFloat = func() float64 { return rand.Float64() }
	}
	return randomVectorInside(fsconf.Rect, fsconf.RandomFloat)
}

type ForceSimulation struct {
	conf        ForceSimulationConfig
	temperature float64
}

type Stats struct {
	Iterations int
	TotalTime  time.Duration
}

func (fs *ForceSimulation) ComputeLayout(ctx context.Context, nodes []*Node, edges []*Edge) ([]*Node, Stats) {
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
simulation:
	for {
		select {
		case <-ctx.Done():
			break simulation
		default:
			// continue looping
		}
		graph.ApplyForce(fs.conf.FrameTime, qt)
		stats.Iterations += 1
		fs.temperature += (fs.conf.AlphaTarget - fs.temperature) * fs.conf.AlphaDecay
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

// calculateRepulsionForce calculates the repulsion force between bodies b1 and
// b2 and adds the result to the totalForce vector
func (fs *ForceSimulation) calculateRepulsionForce(totalForce, tmp *vector.Vector, b1 Body, b2 Body) {
	// this is basically tmp := b1.position().Sub(b2.position()), but allocations are heavy!
	vector.In(*tmp).Sub(*tmp).Add(b1.position()).Sub(b2.position())
	dist := tmp.Magnitude()
	if dist*dist < b1.size()*b2.size() {
		dist = b1.size() * b2.size()
	}
	scale := b1.size() * b2.size() * fs.temperature / dist * fs.conf.RepulsionMultiplier
	vector.In(*tmp).Unit().Scale(scale)
	vector.In(*totalForce).Add(*tmp)
}

func (fs *ForceSimulation) calculateAttractionForce(from *Node, to *Node, weight float64) vector.Vector {
	delta := from.Pos.Sub(to.Pos)
	dist := clamp(delta.Magnitude(), fs.conf.MinDistanceBeweenNodes, math.Inf(+1))
	s := float64(math.Min(float64(from.radius), float64(to.radius)))
	l := float64(from.radius + to.radius)
	force := delta.Unit().Scale((dist - l) / s * weight * fs.temperature)
	return force
}

// Package balancer provides upstream selection strategies (round-robin,
// weighted, random) and passive health tracking for a route's upstream pool.
//
// It is intentionally self-contained (stdlib only) so it can be shared by the
// app (route building) and proxy (request forwarding) layers without creating
// import cycles.
package balancer

import (
	"math/rand/v2"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// Strategy selects how the balancer picks the next upstream from the pool.
type Strategy string

const (
	// RoundRobin cycles through upstreams in order, skipping unhealthy ones.
	RoundRobin Strategy = "round_robin"
	// Weighted uses smooth weighted round-robin honouring each upstream's weight.
	Weighted Strategy = "weighted"
	// Random picks a uniformly random healthy upstream.
	Random Strategy = "random"
)

// Upstream is a single backend target with its load-balancing weight and
// passive health state. Health fields are accessed atomically so a single
// Upstream can be shared across concurrent requests.
type Upstream struct {
	URL    *url.URL
	Weight int

	// consecFailures counts consecutive failed requests since the last success.
	consecFailures atomic.Uint32
	// downUntilNanos is the unix-nano timestamp until which the upstream is
	// considered unhealthy. Zero means healthy.
	downUntilNanos atomic.Int64
}

// String returns the upstream's URL as a string.
func (u *Upstream) String() string {
	if u == nil || u.URL == nil {
		return ""
	}
	return u.URL.String()
}

// Down reports, without mutating state, whether the upstream is currently in a
// cooldown (unhealthy) window. Intended for read-only status reporting.
func (u *Upstream) Down() bool {
	until := u.downUntilNanos.Load()
	return until != 0 && time.Now().UnixNano() < until
}

// healthy reports whether the upstream can receive traffic at the given time.
// When the cooldown window has elapsed it transitions back to healthy
// (half-open) so a single request can probe recovery.
func (u *Upstream) healthy(now time.Time) bool {
	until := u.downUntilNanos.Load()
	if until == 0 {
		return true
	}
	if now.UnixNano() < until {
		return false
	}
	// Cooldown elapsed: half-open. Let one goroutine reset the state and probe.
	if u.downUntilNanos.CompareAndSwap(until, 0) {
		u.consecFailures.Store(0)
	}
	return true
}

// PassiveHealth configures failure-driven ejection of upstreams.
type PassiveHealth struct {
	Enabled          bool
	FailureThreshold uint32
	Cooldown         time.Duration
}

// Balancer chooses upstreams for a single route according to its strategy and
// tracks passive health. A Balancer is safe for concurrent use.
type Balancer struct {
	upstreams []*Upstream
	strategy  Strategy
	passive   PassiveHealth

	rrCounter atomic.Uint64

	// mu guards the smooth weighted round-robin running weights.
	mu    sync.Mutex
	currW []int

	// now is injected for deterministic testing; defaults to time.Now.
	now func() time.Time
}

// New builds a Balancer for the given upstreams. The upstreams slice must be
// non-empty; callers (config validation) guarantee this.
func New(upstreams []*Upstream, strategy Strategy, passive PassiveHealth) *Balancer {
	if strategy == "" {
		strategy = RoundRobin
	}
	return &Balancer{
		upstreams: upstreams,
		strategy:  strategy,
		passive:   passive,
		currW:     make([]int, len(upstreams)),
		now:       time.Now,
	}
}

// Upstreams returns the configured upstream pool (read-only use).
func (b *Balancer) Upstreams() []*Upstream {
	return b.upstreams
}

// Next returns the next healthy upstream according to the strategy, or nil when
// every upstream is currently unhealthy.
func (b *Balancer) Next() *Upstream {
	if len(b.upstreams) == 0 {
		return nil
	}
	now := b.now()
	switch b.strategy {
	case Weighted:
		return b.nextWeighted(now)
	case Random:
		return b.nextRandom(now)
	default:
		return b.nextRoundRobin(now)
	}
}

func (b *Balancer) nextRoundRobin(now time.Time) *Upstream {
	n := uint64(len(b.upstreams))
	// Reduce into [0,n) before adding i so base+i < 2n — safe from uint64 overflow.
	base := b.rrCounter.Add(1) % n
	for i := uint64(0); i < n; i++ {
		idx := base + i
		if idx >= n {
			idx -= n
		}
		if b.upstreams[idx].healthy(now) {
			return b.upstreams[idx]
		}
	}
	return nil
}

// nextWeighted implements nginx-style smooth weighted round-robin restricted to
// healthy upstreams.
func (b *Balancer) nextWeighted(now time.Time) *Upstream {
	b.mu.Lock()
	defer b.mu.Unlock()

	total := 0
	best := -1
	for i, u := range b.upstreams {
		if !u.healthy(now) {
			continue
		}
		w := u.Weight
		if w <= 0 {
			w = 1
		}
		b.currW[i] += w
		total += w
		if best == -1 || b.currW[i] > b.currW[best] {
			best = i
		}
	}
	if best == -1 {
		return nil
	}
	b.currW[best] -= total
	return b.upstreams[best]
}

func (b *Balancer) nextRandom(now time.Time) *Upstream {
	healthy := make([]*Upstream, 0, len(b.upstreams))
	for _, u := range b.upstreams {
		if u.healthy(now) {
			healthy = append(healthy, u)
		}
	}
	if len(healthy) == 0 {
		return nil
	}
	return healthy[rand.IntN(len(healthy))]
}

// RecordResult updates passive health for an upstream based on the outcome of a
// request. It is a no-op when passive health is disabled.
func (b *Balancer) RecordResult(u *Upstream, failed bool) {
	if u == nil || !b.passive.Enabled {
		return
	}
	if !failed {
		u.consecFailures.Store(0)
		return
	}
	if b.passive.FailureThreshold == 0 {
		return
	}
	if u.consecFailures.Add(1) >= b.passive.FailureThreshold {
		u.downUntilNanos.Store(b.now().Add(b.passive.Cooldown).UnixNano())
	}
}

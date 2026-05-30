package balancer

import (
	"net/url"
	"testing"
	"time"
)

func mustUpstreams(t *testing.T, specs ...struct {
	raw    string
	weight int
}) []*Upstream {
	t.Helper()
	ups := make([]*Upstream, 0, len(specs))
	for _, s := range specs {
		u, err := url.Parse(s.raw)
		if err != nil {
			t.Fatalf("parse upstream %q: %v", s.raw, err)
		}
		ups = append(ups, &Upstream{URL: u, Weight: s.weight})
	}
	return ups
}

func up(raw string, weight int) struct {
	raw    string
	weight int
} {
	return struct {
		raw    string
		weight int
	}{raw, weight}
}

func TestRoundRobinDistributesEvenly(t *testing.T) {
	ups := mustUpstreams(t, up("http://a", 1), up("http://b", 1), up("http://c", 1))
	b := New(ups, RoundRobin, PassiveHealth{})

	counts := map[string]int{}
	for i := 0; i < 300; i++ {
		counts[b.Next().String()]++
	}

	for _, u := range ups {
		if got := counts[u.String()]; got != 100 {
			t.Fatalf("expected even distribution of 100 for %s, got %d", u.String(), got)
		}
	}
}

func TestWeightedHonoursWeights(t *testing.T) {
	ups := mustUpstreams(t, up("http://a", 3), up("http://b", 1))
	b := New(ups, Weighted, PassiveHealth{})

	counts := map[string]int{}
	for i := 0; i < 8; i++ {
		counts[b.Next().String()]++
	}

	if counts["http://a"] != 6 || counts["http://b"] != 2 {
		t.Fatalf("expected 3:1 distribution (a=6,b=2), got a=%d b=%d", counts["http://a"], counts["http://b"])
	}
}

func TestRandomReturnsOnlyHealthyAndCoversPool(t *testing.T) {
	ups := mustUpstreams(t, up("http://a", 1), up("http://b", 1))
	b := New(ups, Random, PassiveHealth{})

	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		seen[b.Next().String()] = true
	}
	if !seen["http://a"] || !seen["http://b"] {
		t.Fatalf("expected random to eventually pick both upstreams, saw %v", seen)
	}
}

func TestNextSkipsUnhealthyUpstream(t *testing.T) {
	ups := mustUpstreams(t, up("http://a", 1), up("http://b", 1))
	b := New(ups, RoundRobin, PassiveHealth{Enabled: true, FailureThreshold: 1, Cooldown: time.Hour})

	// Eject upstream a.
	b.RecordResult(ups[0], true)

	for i := 0; i < 50; i++ {
		if got := b.Next(); got != ups[1] {
			t.Fatalf("expected only healthy upstream b, got %s", got.String())
		}
	}
}

func TestNextReturnsNilWhenAllUnhealthy(t *testing.T) {
	ups := mustUpstreams(t, up("http://a", 1))
	b := New(ups, RoundRobin, PassiveHealth{Enabled: true, FailureThreshold: 1, Cooldown: time.Hour})

	b.RecordResult(ups[0], true)

	if got := b.Next(); got != nil {
		t.Fatalf("expected nil when all upstreams unhealthy, got %s", got.String())
	}
}

func TestHalfOpenRecoversAfterCooldown(t *testing.T) {
	ups := mustUpstreams(t, up("http://a", 1))
	b := New(ups, RoundRobin, PassiveHealth{Enabled: true, FailureThreshold: 1, Cooldown: 10 * time.Second})

	base := time.Unix(1700000000, 0)
	b.now = func() time.Time { return base }

	b.RecordResult(ups[0], true)
	if got := b.Next(); got != nil {
		t.Fatalf("expected upstream to be ejected during cooldown, got %s", got.String())
	}

	// Advance time past the cooldown window.
	b.now = func() time.Time { return base.Add(11 * time.Second) }
	if got := b.Next(); got != ups[0] {
		t.Fatalf("expected upstream to recover (half-open) after cooldown, got nil")
	}
}

func TestRecordResultNoOpWhenPassiveDisabled(t *testing.T) {
	ups := mustUpstreams(t, up("http://a", 1))
	b := New(ups, RoundRobin, PassiveHealth{Enabled: false, FailureThreshold: 1, Cooldown: time.Hour})

	b.RecordResult(ups[0], true)

	if got := b.Next(); got != ups[0] {
		t.Fatalf("expected upstream to stay healthy when passive health disabled, got nil")
	}
}

func TestRoundRobinCoversAllUpstreamsAtWrapBoundary(t *testing.T) {
	// Seed counter near MaxUint64 to trigger the former overflow path.
	// n=3 does not divide 2^64 evenly, so the old `(start+i)%n` code would
	// revisit index 0 twice and skip index 2 near the wrap boundary.
	ups := mustUpstreams(t, up("http://a", 1), up("http://b", 1), up("http://c", 1))
	b := New(ups, RoundRobin, PassiveHealth{})
	// Set counter so the next Add(1) gives MaxUint64 - 1, well within n-1 of MaxUint64.
	b.rrCounter.Store(^uint64(0) - 2) // next Add(1) = MaxUint64-1, then MaxUint64, then 0

	covered := map[string]bool{}
	for i := 0; i < 30; i++ {
		u := b.Next()
		if u == nil {
			t.Fatal("round-robin returned nil with all upstreams healthy near wrap boundary")
		}
		covered[u.String()] = true
	}
	for _, u := range ups {
		if !covered[u.String()] {
			t.Fatalf("upstream %s was never selected near uint64 wrap boundary", u.String())
		}
	}
}

func TestRecordResultResetsAfterSuccess(t *testing.T) {
	ups := mustUpstreams(t, up("http://a", 1), up("http://b", 1))
	b := New(ups, RoundRobin, PassiveHealth{Enabled: true, FailureThreshold: 2, Cooldown: time.Hour})

	b.RecordResult(ups[0], true)  // 1 failure, below threshold
	b.RecordResult(ups[0], false) // success resets counter
	b.RecordResult(ups[0], true)  // 1 failure again, still below threshold

	if ups[0].Down() {
		t.Fatal("expected upstream to remain healthy: success should reset failure streak")
	}
}

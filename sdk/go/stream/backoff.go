package stream

import (
	"math/rand"
	"sync"
	"time"
)

type BackoffConfig struct {
	Min    time.Duration
	Max    time.Duration
	Jitter float64
}

func (c BackoffConfig) withDefaults() BackoffConfig {
	if c.Min <= 0 {
		c.Min = 250 * time.Millisecond
	}
	if c.Max <= 0 {
		c.Max = 5 * time.Second
	}
	if c.Max < c.Min {
		c.Max = c.Min
	}
	if c.Jitter < 0 {
		c.Jitter = 0
	}
	if c.Jitter > 1 {
		c.Jitter = 1
	}
	return c
}

type Backoff struct {
	cfg     BackoffConfig
	attempt int
	rngMu   sync.Mutex
	rng     *rand.Rand
}

func NewBackoff(cfg BackoffConfig) *Backoff {
	return &Backoff{
		cfg: cfg.withDefaults(),
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (b *Backoff) Next() time.Duration {
	base := b.cfg.Min
	for i := 0; i < b.attempt; i++ {
		if base >= b.cfg.Max/2 {
			base = b.cfg.Max
			break
		}
		base *= 2
	}
	b.attempt++

	if b.cfg.Jitter == 0 {
		return base
	}

	delta := float64(base) * b.cfg.Jitter
	min := float64(base) - delta
	max := float64(base) + delta

	b.rngMu.Lock()
	factor := b.rng.Float64()
	b.rngMu.Unlock()

	return time.Duration(min + (max-min)*factor)
}

func (b *Backoff) Reset() {
	b.attempt = 0
}

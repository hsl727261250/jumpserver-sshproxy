package session

import "sync/atomic"

// Counter tracks authenticated SSH connections in this process.
// The count is not stored in Core and is lost when the process exits.
type Counter struct {
	n atomic.Int64
}

func (c *Counter) Enter() {
	c.n.Add(1)
}

func (c *Counter) Leave() {
	c.n.Add(-1)
}

func (c *Counter) Online() int {
	n := c.n.Load()
	if n < 0 {
		return 0
	}
	return int(n)
}

var active Counter

func Enter() { active.Enter() }

func Leave() { active.Leave() }

func Online() int { return active.Online() }

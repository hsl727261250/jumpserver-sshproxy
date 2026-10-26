package session

import (
	"sync"
	"sync/atomic"

	"github.com/jumpserver/jumpserver-sshproxy/internal/logger"
)

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

type tracked struct {
	session *Session
	close   func()
	once    sync.Once
}

var registry = struct {
	mu    sync.Mutex
	items map[string]*tracked
}{items: map[string]*tracked{}}

// Track remembers a Core session until Untrack. close runs once from Terminate.
func Track(id string, s *Session, close func()) {
	if id == "" {
		return
	}
	registry.mu.Lock()
	defer registry.mu.Unlock()
	registry.items[id] = &tracked{session: s, close: close}
}

func Untrack(id string) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	delete(registry.items, id)
}

func IDs() []string {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	ids := make([]string, 0, len(registry.items))
	for id := range registry.items {
		ids = append(ids, id)
	}
	return ids
}

// Terminate marks the session as an administrator disconnect and closes it once.
func Terminate(id string) bool {
	registry.mu.Lock()
	item := registry.items[id]
	registry.mu.Unlock()
	if item == nil {
		return false
	}
	ran := false
	item.once.Do(func() {
		ran = true
		user := ""
		if item.session != nil {
			item.session.Finish(ReasonAdminTerminate)
			user = item.session.User
		}
		if item.close != nil {
			logger.Infof("SSH session closed user=%s reason=%s core_session=%s", user, ReasonAdminTerminate, id)
			item.close()
		}
	})
	return ran
}

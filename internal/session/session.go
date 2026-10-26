package session

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jumpserver/jumpserver-sshproxy/internal/logger"
)

const (
	ReasonClientDisconnect = "client_disconnect"
	ReasonIdleDisconnect   = "idle_disconnect"
	ReasonMaxSession       = "max_session_timeout"
	ReasonAdminTerminate   = "admin_terminate"
)

type ctxKey struct{}

type Session struct {
	User    string
	Started time.Time

	last   atomic.Int64
	mu     sync.Mutex
	reason string
}

func New(user string) *Session {
	s := &Session{User: user, Started: time.Now()}
	s.Touch()
	return s
}

func Store(setter interface{ SetValue(any, any) }, s *Session) {
	setter.SetValue(ctxKey{}, s)
}

func From(value interface{ Value(any) any }) *Session {
	if value == nil {
		return nil
	}
	s, _ := value.Value(ctxKey{}).(*Session)
	return s
}

func (s *Session) Touch() {
	if s == nil {
		return
	}
	s.last.Store(time.Now().UnixNano())
}

func (s *Session) Due(now time.Time, idle, max time.Duration) string {
	if s == nil {
		return ""
	}
	if max > 0 && now.Sub(s.Started) >= max {
		return ReasonMaxSession
	}
	last := time.Unix(0, s.last.Load())
	if idle > 0 && now.Sub(last) >= idle {
		return ReasonIdleDisconnect
	}
	return ""
}

func (s *Session) Finish(reason string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reason != "" {
		return ""
	}
	s.reason = reason
	return reason
}

func (s *Session) Reason() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reason
}

func Watch(ctx context.Context, s *Session, idle, max time.Duration, closer func() io.Closer) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if reason := s.Finish(ReasonClientDisconnect); reason != "" {
				logger.Infof("SSH session closed user=%s reason=%s", s.User, reason)
			}
			return
		case now := <-ticker.C:
			reason := s.Due(now, idle, max)
			if reason == "" {
				continue
			}
			if finished := s.Finish(reason); finished != "" {
				logger.Infof("SSH session closed user=%s reason=%s", s.User, finished)
			}
			if closer != nil {
				if conn := closer(); conn != nil {
					_ = conn.Close()
				}
			}
			return
		}
	}
}

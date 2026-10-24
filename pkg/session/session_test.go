package session

import (
	"testing"
	"time"
)

func TestDue(t *testing.T) {
	s := New("alice")
	if reason := s.Due(s.Started.Add(time.Minute), 0, 0); reason != "" {
		t.Fatalf("disabled limits returned %s", reason)
	}
	if reason := s.Due(s.Started.Add(31*time.Minute), 30*time.Minute, 24*time.Hour); reason != ReasonIdleDisconnect {
		t.Fatalf("idle reason %s", reason)
	}
	if reason := s.Due(s.Started.Add(25*time.Hour), 0, 24*time.Hour); reason != ReasonMaxSession {
		t.Fatalf("max reason %s", reason)
	}
	if got := s.Finish(ReasonClientDisconnect); got != ReasonClientDisconnect {
		t.Fatalf("finish %s", got)
	}
	if got := s.Finish(ReasonIdleDisconnect); got != "" || s.Reason() != ReasonClientDisconnect {
		t.Fatalf("second finish %s reason %s", got, s.Reason())
	}
}

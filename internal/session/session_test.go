package session_test

import (
	"testing"
	"time"

	"github.com/jumpserver/jumpserver-sshproxy/internal/session"
)

func TestDue(t *testing.T) {
	s := session.New("alice")
	if reason := s.Due(s.Started.Add(time.Minute), 0, 0); reason != "" {
		t.Fatalf("disabled limits returned %s", reason)
	}
	if reason := s.Due(s.Started.Add(31*time.Minute), 30*time.Minute, 24*time.Hour); reason != session.ReasonIdleDisconnect {
		t.Fatalf("idle reason %s", reason)
	}
	if reason := s.Due(s.Started.Add(25*time.Hour), 0, 24*time.Hour); reason != session.ReasonMaxSession {
		t.Fatalf("max reason %s", reason)
	}
	if got := s.Finish(session.ReasonClientDisconnect); got != session.ReasonClientDisconnect {
		t.Fatalf("finish %s", got)
	}
	if got := s.Finish(session.ReasonIdleDisconnect); got != "" || s.Reason() != session.ReasonClientDisconnect {
		t.Fatalf("second finish %s reason %s", got, s.Reason())
	}
}

func TestCounter(t *testing.T) {
	var counter session.Counter
	if counter.Online() != 0 {
		t.Fatalf("online %d", counter.Online())
	}
	counter.Enter()
	counter.Enter()
	if counter.Online() != 2 {
		t.Fatalf("online %d", counter.Online())
	}
	counter.Leave()
	if counter.Online() != 1 {
		t.Fatalf("online %d", counter.Online())
	}
	counter.Leave()
	counter.Leave()
	if counter.Online() != 0 {
		t.Fatalf("online %d", counter.Online())
	}
}

func TestTerminateClosesOnce(t *testing.T) {
	s := session.New("alice")
	closed := 0
	const id = "registry-test-session"
	session.Track(id, s, func() { closed++ })
	t.Cleanup(func() { session.Untrack(id) })
	if !session.Terminate(id) || closed != 1 || s.Reason() != session.ReasonAdminTerminate {
		t.Fatalf("closed %d reason %s", closed, s.Reason())
	}
	if session.Terminate(id) || closed != 1 {
		t.Fatalf("second close %d", closed)
	}
	found := false
	for _, got := range session.IDs() {
		if got == id {
			found = true
		}
	}
	if !found {
		t.Fatal("session id was removed before untrack")
	}
	session.Untrack(id)
	if session.Terminate(id) {
		t.Fatal("terminated a removed session")
	}
}

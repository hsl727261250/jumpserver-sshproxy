package sshproxy_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gliderlabs/ssh"

	"github.com/jumpserver/jumpserver-sshproxy/pkg/auth"
	"github.com/jumpserver/jumpserver-sshproxy/pkg/jmsauth"
	"github.com/jumpserver/jumpserver-sshproxy/pkg/session"
)

func TestPasswordSuccess(t *testing.T) {
	core := coreWith(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusCreated, map[string]any{
			"token": "token-1",
			"user":  map[string]string{"id": "u1", "username": "alice"},
		})
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fake := newFake(ctx, "alice")
	err := auth.NewAuthenticator(core, 0, 0).Password(fake, "secret")
	if err != nil {
		t.Fatal(err)
	}
	if session.From(fake) == nil || session.From(fake).User != "alice" {
		t.Fatal("session was not stored")
	}
}

func TestOTPRequiredAndRejected(t *testing.T) {
	core := coreWith(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/authentication/mfa/challenge/":
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "mfa_failed", "msg": "invalid"})
		default:
			writeJSON(w, http.StatusOK, map[string]any{
				"error": "mfa_required",
				"data":  map[string]any{"choices": []string{"otp"}},
			})
		}
	})
	fake := newFake(context.Background(), "alice")
	err := auth.NewAuthenticator(core, 0, 0).Password(fake, "secret")
	partial, ok := err.(*ssh.PartialSuccessError)
	if !ok || partial.Next.KeyboardInteractiveCallback == nil {
		t.Fatalf("expected partial success, got %v", err)
	}
	err = partial.Next.KeyboardInteractiveCallback(fake, func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		return []string{"000000"}, nil
	})
	if err == nil {
		t.Fatal("expected otp rejection")
	}
}

type fakeCtx struct {
	context.Context
	sync.Mutex
	user   string
	values map[any]any
}

func newFake(ctx context.Context, user string) *fakeCtx {
	return &fakeCtx{Context: ctx, user: user, values: map[any]any{}}
}

func (f *fakeCtx) User() string          { return f.user }
func (f *fakeCtx) SessionID() string     { return "session-1" }
func (f *fakeCtx) ClientVersion() string { return "test" }
func (f *fakeCtx) ServerVersion() string { return "test" }
func (f *fakeCtx) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("203.0.113.8"), Port: 1234}
}
func (f *fakeCtx) LocalAddr() net.Addr           { return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 1024} }
func (f *fakeCtx) Permissions() *ssh.Permissions { return nil }

func (f *fakeCtx) Value(key any) any {
	f.Lock()
	defer f.Unlock()
	if value, ok := f.values[key]; ok {
		return value
	}
	return f.Context.Value(key)
}

func (f *fakeCtx) SetValue(key, value any) {
	f.Lock()
	defer f.Unlock()
	f.values[key] = value
}

func coreWith(t *testing.T, handler http.HandlerFunc) *jmsauth.Core {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return jmsauth.NewCore(server.URL, testKey(), time.Second)
}

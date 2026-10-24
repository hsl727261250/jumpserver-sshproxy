package sshd

import (
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/jumpserver/jumpserver-sshproxy/pkg/config"
	"github.com/jumpserver/jumpserver-sshproxy/pkg/jmsauth"
)

func TestAllowLocalForwardRequiresSession(t *testing.T) {
	if AllowLocalForward(nil, "10.0.0.8", 5432) {
		t.Fatal("nil context allowed forwarding")
	}
}

func TestForwardAfterPasswordAuth(t *testing.T) {
	coreServer := httptestServer(t)
	dir := t.TempDir()
	config.GlobalConfig = &config.Config{
		HostKeyFilePath:    filepath.Join(dir, "host_key"),
		ForwardDialTimeout: 5,
	}
	t.Cleanup(func() { config.GlobalConfig = nil })

	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		conn, err := echo.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = io.Copy(conn, conn)
	}()

	server := NewSSHServer(coreServer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() { _ = server.Srv.Serve(ln) }()
	defer func() { _ = server.Srv.Close() }()

	client, err := gossh.Dial("tcp", ln.Addr().String(), &gossh.ClientConfig{
		User:            "alice",
		Auth:            []gossh.AuthMethod{gossh.Password("secret")},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	forwarded, err := client.Dial("tcp", echo.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer forwarded.Close()
	_ = forwarded.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err = forwarded.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err = io.ReadFull(forwarded, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "ping" {
		t.Fatalf("echo %q", buf)
	}
}

func httptestServer(t *testing.T) *jmsauth.Core {
	t.Helper()
	server := newHTTPServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"token-1","user":{"id":"u1","username":"alice"}}`))
	})
	return jmsauth.NewCore(server, jmsauth.AccessKey{
		ID:     "test-ak",
		Secret: "0123456789abcdef0123456789abcdef",
	}, time.Second)
}

func newHTTPServer(t *testing.T, handler http.HandlerFunc) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: handler}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return "http://" + ln.Addr().String()
}

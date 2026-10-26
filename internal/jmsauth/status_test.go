package jmsauth_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jumpserver/jumpserver-sshproxy/internal/jmsauth"
)

const statusPath = "/api/v1/terminal/status/"

type statusBody struct {
	CPULoad       float64  `json:"cpu_load"`
	MemoryUsed    float64  `json:"memory_used"`
	DiskUsed      float64  `json:"disk_used"`
	SessionOnline int      `json:"session_online"`
	Sessions      []string `json:"sessions"`
}

func TestReportStatus(t *testing.T) {
	var seen struct {
		authorization string
		date          string
		body          statusBody
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != statusPath {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		seen.authorization = r.Header.Get("Authorization")
		seen.date = r.Header.Get("Date")
		content, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(content, &seen.body); err != nil {
			t.Errorf("decode %s", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	key := testKey()
	core := jmsauth.NewCore(server.URL, key, time.Second)
	if _, err := core.ReportStatus(t.Context(), 3, nil); err != nil {
		t.Fatal(err)
	}
	if seen.date == "" || !strings.HasSuffix(seen.date, "GMT") {
		t.Fatalf("date %q", seen.date)
	}
	signing := fmt.Sprintf("(request-target): post %s\ndate: %s", statusPath, seen.date)
	mac := hmac.New(sha256.New, []byte(key.Secret))
	mac.Write([]byte(signing))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if !strings.Contains(seen.authorization, `keyId="test-ak"`) || !strings.Contains(seen.authorization, want) {
		t.Fatalf("authorization %q", seen.authorization)
	}
	if seen.body.SessionOnline != 3 {
		t.Fatalf("sessions %d", seen.body.SessionOnline)
	}
	if seen.body.Sessions == nil || len(seen.body.Sessions) != 0 {
		t.Fatalf("session ids %+v", seen.body.Sessions)
	}
	if seen.body.MemoryUsed < 0 || seen.body.DiskUsed < 0 || seen.body.CPULoad < 0 {
		t.Fatalf("stat %+v", seen.body)
	}
}

func TestReportStatusHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "denied", http.StatusForbidden)
	}))
	defer server.Close()

	core := jmsauth.NewCore(server.URL, testKey(), time.Second)
	if _, err := core.ReportStatus(t.Context(), 0, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestKeepAliveStops(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	core := jmsauth.NewCore(server.URL, testKey(), time.Second)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		core.KeepAlive(ctx, func() (int, []string) { return 1, nil }, nil)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for hits.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("keepalive did not stop")
	}
	if hits.Load() == 0 {
		t.Fatal("status was not reported")
	}
}

func TestReportStatusSendsSessionIDs(t *testing.T) {
	var seen statusBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(content, &seen); err != nil {
			t.Errorf("decode %s", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	core := jmsauth.NewCore(server.URL, testKey(), time.Second)
	if _, err := core.ReportStatus(t.Context(), 2, []string{"s1", "s2"}); err != nil {
		t.Fatal(err)
	}
	if seen.SessionOnline != 2 || len(seen.Sessions) != 2 || seen.Sessions[0] != "s1" || seen.Sessions[1] != "s2" {
		t.Fatalf("body %+v", seen)
	}
}

func TestKeepAliveKillSession(t *testing.T) {
	var killed atomic.Bool
	var finished atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == statusPath:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`[{"id":"task-lock","name":"lock_session","args":"sess-lock"},{"id":"task-1","name":"kill_session","args":"sess-1"}]`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/terminal/tasks/task-1/":
			finished.Store(true)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	core := jmsauth.NewCore(server.URL, testKey(), time.Second)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		core.KeepAlive(ctx, func() (int, []string) { return 1, []string{"sess-1"} }, func(id string) {
			if id == "sess-1" {
				killed.Store(true)
			}
		})
	}()
	deadline := time.Now().Add(2 * time.Second)
	for !finished.Load() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("keepalive did not stop")
	}
	if !killed.Load() || !finished.Load() {
		t.Fatalf("killed %v finished %v", killed.Load(), finished.Load())
	}
}

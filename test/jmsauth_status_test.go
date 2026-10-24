package sshproxy_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jumpserver/jumpserver-sshproxy/pkg/jmsauth"
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
		header string
		body   statusBody
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != statusPath {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		seen.header = r.Header.Get("X-JMS-SVC")
		content, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(content, &seen.body); err != nil {
			t.Errorf("decode %s", err)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("[]"))
	}))
	defer server.Close()

	core := jmsauth.NewCore(server.URL, testKey(), time.Second)
	if err := core.ReportStatus(t.Context(), 3); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(seen.header, "Sign test-ak:") {
		t.Fatalf("signature %q", seen.header)
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
	if err := core.ReportStatus(t.Context(), 0); err == nil {
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
		core.KeepAlive(ctx, func() int { return 1 })
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

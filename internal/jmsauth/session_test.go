package jmsauth_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jumpserver/jumpserver-sshproxy/internal/jmsauth"
)

func TestCreateAndFinishSession(t *testing.T) {
	var created map[string]any
	var finished map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content, _ := io.ReadAll(r.Body)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/terminal/sessions/":
			if err := json.Unmarshal(content, &created); err != nil {
				t.Errorf("decode create %s", err)
			}
			writeJSON(w, http.StatusCreated, map[string]string{"id": "core-sess"})
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/terminal/sessions/core-sess/":
			if err := json.Unmarshal(content, &finished); err != nil {
				t.Errorf("decode finish %s", err)
			}
			writeJSON(w, http.StatusOK, map[string]string{"id": "core-sess"})
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	core := jmsauth.NewCore(server.URL, testKey(), time.Second)
	id, err := core.CreateSession(t.Context(), jmsauth.SessionSpec{
		User:       "alice",
		UserID:     "user-1",
		AssetID:    "asset-1",
		RemoteAddr: "203.0.113.8",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "core-sess" {
		t.Fatalf("id %s", id)
	}
	if created["protocol"] != "proxy" || created["type"] != "tunnel" || created["login_from"] != "ST" {
		t.Fatalf("create %+v", created)
	}
	if created["user"] != "alice" || created["user_id"] != "user-1" || created["asset_id"] != "asset-1" {
		t.Fatalf("identity %+v", created)
	}
	if created["account"] != "alice" || created["account_id"] != "user-1" || created["remote_addr"] != "203.0.113.8" {
		t.Fatalf("account %+v", created)
	}
	if created["has_replay"] != false || created["error_reason"] != "replay_unsupported" || created["is_finished"] != false {
		t.Fatalf("replay %+v", created)
	}
	if err = core.FinishSession(t.Context(), id); err != nil {
		t.Fatal(err)
	}
	if finished["is_finished"] != true || finished["date_end"] == "" {
		t.Fatalf("finish %+v", finished)
	}
}

func TestCreateSessionHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no asset", http.StatusBadRequest)
	}))
	defer server.Close()

	core := jmsauth.NewCore(server.URL, testKey(), time.Second)
	if _, err := core.CreateSession(t.Context(), jmsauth.SessionSpec{User: "alice", UserID: "user-1", AssetID: "missing"}); err == nil {
		t.Fatal("expected error")
	}
}

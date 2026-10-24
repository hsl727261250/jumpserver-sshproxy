package jmsauth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testKey() AccessKey {
	return AccessKey{ID: "test-ak", Secret: "0123456789abcdef0123456789abcdef"}
}

func TestAuthenticatePasswordAndSignature(t *testing.T) {
	var seen SignCapture
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.record(r)
		if r.URL.Path != "/api/v1/authentication/tokens/" {
			t.Fatalf("path %s", r.URL.Path)
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"token": "token-1",
			"user":  map[string]string{"id": "u1", "username": "alice"},
		})
	}))
	defer server.Close()

	core := NewCore(server.URL, testKey(), time.Second)
	client, err := core.UserClient()
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Authenticate(t.Context(), "alice", "secret", "203.0.113.8")
	if err != nil {
		t.Fatal(err)
	}
	if result.Phase != PhaseSuccess || result.User.ID != "u1" {
		t.Fatalf("result %+v", result)
	}
	if !seen.hasPrefix("X-Jms-Svc", "Sign test-ak:") {
		t.Fatalf("signature %q", seen.header("X-Jms-Svc"))
	}
	if seen.body["remote_addr"] != "203.0.113.8" || seen.body["password"] != "secret" || seen.body["login_type"] != "T" {
		t.Fatalf("body %+v", seen.body)
	}
}

func TestAuthenticateOTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/authentication/tokens/":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["password"] != "" {
				http.SetCookie(w, &http.Cookie{Name: "sessionid", Value: "abc", Path: "/"})
				writeJSON(w, http.StatusOK, map[string]any{
					"error": "mfa_required",
					"data":  map[string]any{"choices": []string{"otp", "face"}},
				})
				return
			}
			if r.Header.Get("Cookie") == "" {
				t.Fatal("missing session cookie")
			}
			writeJSON(w, http.StatusCreated, map[string]any{
				"token": "token-2",
				"user":  map[string]string{"id": "u1", "username": "alice"},
			})
		case "/api/v1/authentication/mfa/challenge/":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["code"] != "654321" || body["type"] != "otp" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "mfa_failed"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"msg": "ok"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	core := NewCore(server.URL, testKey(), time.Second)
	client, err := core.UserClient()
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Authenticate(t.Context(), "alice", "secret", "203.0.113.8")
	if err != nil {
		t.Fatal(err)
	}
	if result.Phase != PhaseMFA {
		t.Fatalf("phase %v", result.Phase)
	}
	if err = client.VerifyMFA(t.Context(), "otp", "654321"); err != nil {
		t.Fatal(err)
	}
	result, err = client.Authenticate(t.Context(), "alice", "", "203.0.113.8")
	if err != nil {
		t.Fatal(err)
	}
	if result.Phase != PhaseSuccess || result.User.Username != "alice" {
		t.Fatalf("result %+v", result)
	}

	bad, err := core.UserClient()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = bad.Authenticate(t.Context(), "alice", "secret", "203.0.113.8"); err != nil {
		t.Fatal(err)
	}
	if err = bad.VerifyMFA(t.Context(), "otp", "000000"); err == nil {
		t.Fatal("expected mfa failure")
	}
}

func TestRegister(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bootstrap secret-token" {
			t.Fatalf("authorization %q", r.Header.Get("Authorization"))
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["type"] != "koko" || body["name"] != "sshproxy" {
			t.Fatalf("body %+v", body)
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"service_account": map[string]any{
				"access_key": map[string]string{"id": "id-1", "secret": "sec-1"},
			},
		})
	}))
	defer server.Close()

	core := NewCore(server.URL, AccessKey{}, time.Second)
	key, err := core.Register(t.Context(), "sshproxy", "koko", "secret-token")
	if err != nil {
		t.Fatal(err)
	}
	if key.ID != "id-1" || key.Secret != "sec-1" {
		t.Fatalf("key %+v", key)
	}
	path := t.TempDir() + "/.access_key"
	if err = SaveAccessKey(path, key); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadAccessKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != key {
		t.Fatalf("loaded %+v", loaded)
	}
}

type SignCapture struct {
	headerValues http.Header
	body         map[string]string
}

func (s *SignCapture) record(r *http.Request) {
	s.headerValues = r.Header.Clone()
	content, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(content, &s.body)
}

func (s SignCapture) header(name string) string {
	return s.headerValues.Get(name)
}

func (s SignCapture) hasPrefix(name, prefix string) bool {
	value := s.header(name)
	return len(value) >= len(prefix) && value[:len(prefix)] == prefix
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

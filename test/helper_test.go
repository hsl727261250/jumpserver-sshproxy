package sshproxy_test

import (
	"encoding/json"
	"net/http"

	"github.com/jumpserver/jumpserver-sshproxy/pkg/jmsauth"
)

func testKey() jmsauth.AccessKey {
	return jmsauth.AccessKey{ID: "test-ak", Secret: "0123456789abcdef0123456789abcdef"}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

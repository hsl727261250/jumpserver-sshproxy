package jmsauth_test

import (
	"crypto/aes"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/jumpserver/jumpserver-sshproxy/internal/jmsauth"
)

func TestServiceSignatureRoundTrip(t *testing.T) {
	key := jmsauth.AccessKey{ID: "ak", Secret: "0123456789abcdef0123456789abcdef"}
	now := time.Unix(1_700_000_000, 0)
	signature, err := jmsauth.ServiceSignature(key, now)
	if err != nil {
		t.Fatal(err)
	}
	prefix := "Sign ak:"
	if !strings.HasPrefix(signature, prefix) {
		t.Fatalf("signature %q", signature)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(signature, prefix))
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(signatureKey([]byte(strings.ReplaceAll(key.Secret, "-", ""))))
	if err != nil {
		t.Fatal(err)
	}
	plain := make([]byte, len(raw))
	block.Decrypt(plain, raw)
	if got := strings.TrimRight(string(plain), "\x00"); got != "1700000000" {
		t.Fatalf("timestamp %q", got)
	}
}

func signatureKey(key []byte) []byte {
	if len(key) >= 32 {
		out := make([]byte, 32)
		copy(out, key[:32])
		return out
	}
	for len(key)%aes.BlockSize != 0 {
		key = append(key, 0)
	}
	return key
}

package jmsauth

import (
	"crypto/aes"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestServiceSignatureRoundTrip(t *testing.T) {
	key := AccessKey{ID: "ak", Secret: "0123456789abcdef0123456789abcdef"}
	now := time.Unix(1_700_000_000, 0)
	signature, err := ServiceSignature(key, now)
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
	block, err := aes.NewCipher(paddingKey([]byte(key.Secret)))
	if err != nil {
		t.Fatal(err)
	}
	plain := make([]byte, len(raw))
	block.Decrypt(plain, raw)
	if got := strings.TrimRight(string(plain), "\x00"); got != "1700000000" {
		t.Fatalf("timestamp %q", got)
	}
}

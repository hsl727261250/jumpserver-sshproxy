package sshd

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"

	"golang.org/x/crypto/ssh"
)

func LoadOrCreateHostKey(path string) (ssh.Signer, error) {
	content, err := os.ReadFile(path)
	if err == nil {
		return ssh.ParsePrivateKey(content)
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	block, err := ssh.MarshalPrivateKey(privateKey, "")
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(path, pem.EncodeToMemory(block), 0600); err != nil {
		return nil, err
	}
	return ssh.NewSignerFromKey(privateKey)
}

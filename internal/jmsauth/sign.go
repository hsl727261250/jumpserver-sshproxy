package jmsauth

import (
	"crypto/aes"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

func ServiceSignature(key AccessKey, now time.Time) (string, error) {
	secret := strings.ReplaceAll(key.Secret, "-", "")
	block, err := aes.NewCipher(paddingKey([]byte(secret)))
	if err != nil {
		return "", err
	}
	plain := zeroPad([]byte(fmt.Sprintf("%d", now.Unix())))
	encrypted := make([]byte, len(plain))
	for i := 0; i < len(plain); i += aes.BlockSize {
		block.Encrypt(encrypted[i:i+aes.BlockSize], plain[i:i+aes.BlockSize])
	}
	return fmt.Sprintf("Sign %s:%s", key.ID, base64.StdEncoding.EncodeToString(encrypted)), nil
}

func paddingKey(key []byte) []byte {
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

func zeroPad(plain []byte) []byte {
	for len(plain)%aes.BlockSize != 0 {
		plain = append(plain, 0)
	}
	return plain
}

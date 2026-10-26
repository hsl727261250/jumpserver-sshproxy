package jmsauth

import (
	"crypto/aes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
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

const accessKeyDateFormat = "Mon, 02 Jan 2006 15:04:05 GMT"

func AccessKeySignature(key AccessKey, method, path string, now time.Time) (date, authorization string, err error) {
	if key.ID == "" || key.Secret == "" {
		return "", "", fmt.Errorf("access key is empty")
	}
	date = now.UTC().Format(accessKeyDateFormat)
	signing := fmt.Sprintf("(request-target): %s %s\ndate: %s", strings.ToLower(method), path, date)
	mac := hmac.New(sha256.New, []byte(key.Secret))
	mac.Write([]byte(signing))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	authorization = fmt.Sprintf(
		`Signature keyId="%s",algorithm="hmac-sha256",headers="(request-target) date",signature="%s"`,
		key.ID, signature,
	)
	return date, authorization, nil
}

func setAccessKeyAuth(req *http.Request, key AccessKey, now time.Time) error {
	path := req.URL.EscapedPath()
	if req.URL.RawQuery != "" {
		path += "?" + req.URL.RawQuery
	}
	date, authorization, err := AccessKeySignature(key, req.Method, path, now)
	if err != nil {
		return err
	}
	req.Header.Set("Date", date)
	req.Header.Set("Authorization", authorization)
	return nil
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

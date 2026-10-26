package jmsauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type registerResponse struct {
	ServiceAccount struct {
		AccessKey AccessKey `json:"access_key"`
	} `json:"service_account"`
}

func (c *Core) Register(ctx context.Context, name, terminalType, bootstrap string) (AccessKey, error) {
	body, err := json.Marshal(map[string]string{
		"name":    name,
		"type":    terminalType,
		"comment": "sshproxy",
	})
	if err != nil {
		return AccessKey{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Host+"/api/v1/terminal/terminal-registrations/", bytes.NewReader(body))
	if err != nil {
		return AccessKey{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bootstrap "+bootstrap)
	req.Header.Set("User-Agent", "JumpServer-SSHProxy")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return AccessKey{}, err
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return AccessKey{}, err
	}
	if resp.StatusCode >= 300 {
		return AccessKey{}, fmt.Errorf("register terminal: status %d: %s", resp.StatusCode, bytes.TrimSpace(content))
	}
	var parsed registerResponse
	if err = json.Unmarshal(content, &parsed); err != nil {
		return AccessKey{}, err
	}
	key := parsed.ServiceAccount.AccessKey
	if key.ID == "" || key.Secret == "" {
		return AccessKey{}, fmt.Errorf("register terminal: access key is empty")
	}
	return key, nil
}

func (c *Core) EnsureAccessKey(path, name, terminalType, bootstrap string) error {
	if key, err := LoadAccessKey(path); err == nil {
		c.Key = key
		return nil
	}
	if bootstrap == "" {
		return fmt.Errorf("bootstrap token is empty and access key file is missing")
	}
	if terminalType == "" {
		terminalType = "koko"
	}
	var last error
	for i := 0; i < 10; i++ {
		key, err := c.Register(context.Background(), name, terminalType, bootstrap)
		if err == nil {
			if err = SaveAccessKey(path, key); err != nil {
				return err
			}
			c.Key = key
			return nil
		}
		last = err
		if i < 9 {
			time.Sleep(5 * time.Second)
		}
	}
	return last
}

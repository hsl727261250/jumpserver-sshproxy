package jmsauth

import (
	"encoding/json"
	"os"
)

type AccessKey struct {
	ID     string `json:"id"`
	Secret string `json:"secret"`
}

func LoadAccessKey(path string) (AccessKey, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return AccessKey{}, err
	}
	var key AccessKey
	if err = json.Unmarshal(content, &key); err != nil {
		return AccessKey{}, err
	}
	if key.ID == "" || key.Secret == "" {
		return AccessKey{}, os.ErrInvalid
	}
	return key, nil
}

func SaveAccessKey(path string, key AccessKey) error {
	content, err := json.Marshal(key)
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0600)
}

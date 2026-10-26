package jmsauth

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const (
	sessionProtocol    = "proxy"
	sessionLoginFrom   = "ST"
	sessionType        = "tunnel"
	sessionReplayError = "replay_unsupported"

	sessionsPath = "/api/v1/terminal/sessions/"
	tasksPath    = "/api/v1/terminal/tasks/"
)

type SessionSpec struct {
	User       string
	UserID     string
	AssetID    string
	RemoteAddr string
}

type coreSession struct {
	ID          string `json:"id,omitempty"`
	User        string `json:"user,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	Asset       string `json:"asset"`
	AssetID     string `json:"asset_id,omitempty"`
	OrgID       string `json:"org_id"`
	Account     string `json:"account,omitempty"`
	AccountID   string `json:"account_id,omitempty"`
	Protocol    string `json:"protocol,omitempty"`
	LoginFrom   string `json:"login_from,omitempty"`
	Type        string `json:"type,omitempty"`
	RemoteAddr  string `json:"remote_addr,omitempty"`
	IsFinished  bool   `json:"is_finished"`
	HasReplay   bool   `json:"has_replay"`
	ErrorReason string `json:"error_reason,omitempty"`
}

func (c *Core) CreateSession(ctx context.Context, spec SessionSpec) (string, error) {
	body := coreSession{
		User:        spec.User,
		UserID:      spec.UserID,
		Asset:       spec.AssetID,
		AssetID:     spec.AssetID,
		OrgID:       "",
		Account:     spec.User,
		AccountID:   spec.UserID,
		Protocol:    sessionProtocol,
		LoginFrom:   sessionLoginFrom,
		Type:        sessionType,
		RemoteAddr:  spec.RemoteAddr,
		IsFinished:  false,
		HasReplay:   false,
		ErrorReason: sessionReplayError,
	}
	content, _, err := c.signedJSON(ctx, httpMethodPost, sessionsPath, body)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	var created coreSession
	if err = json.Unmarshal(content, &created); err != nil {
		return "", fmt.Errorf("create session: decode response: %w", err)
	}
	if created.ID == "" {
		return "", fmt.Errorf("create session: id is empty")
	}
	return created.ID, nil
}

func (c *Core) FinishSession(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("finish session: id is empty")
	}
	body := struct {
		IsFinished bool   `json:"is_finished"`
		DateEnd    string `json:"date_end"`
	}{
		IsFinished: true,
		DateEnd:    time.Now().UTC().Format(time.RFC3339),
	}
	_, _, err := c.signedJSON(ctx, httpMethodPatch, sessionsPath+id+"/", body)
	if err != nil {
		return fmt.Errorf("finish session: %w", err)
	}
	return nil
}

func (c *Core) FinishTask(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("finish task: id is empty")
	}
	_, _, err := c.signedJSON(ctx, httpMethodPatch, tasksPath+id+"/", map[string]bool{"is_finished": true})
	if err != nil {
		return fmt.Errorf("finish task: %w", err)
	}
	return nil
}

const (
	httpMethodPost  = "POST"
	httpMethodPatch = "PATCH"
)

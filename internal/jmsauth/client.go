package jmsauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"
)

type Phase int

const (
	PhaseFailed Phase = iota
	PhaseSuccess
	PhaseMFA
	PhaseConfirm
)

type User struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

type Result struct {
	Phase   Phase
	User    User
	Choices []string
	Message string
}

type Core struct {
	Host       string
	Key        AccessKey
	Timeout    time.Duration
	HTTPClient *http.Client
}

type UserClient struct {
	core    *Core
	http    *http.Client
	Choices []string
}

func NewCore(host string, key AccessKey, timeout time.Duration) *Core {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Core{
		Host:    strings.TrimRight(host, "/"),
		Key:     key,
		Timeout: timeout,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Core) UserClient() (*UserClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &UserClient{
		core: c,
		http: &http.Client{
			Timeout: c.Timeout,
			Jar:     jar,
		},
	}, nil
}

func (u *UserClient) Authenticate(ctx context.Context, username, password, remoteAddr string) (Result, error) {
	body := map[string]string{
		"username":    username,
		"login_type":  "T",
		"remote_addr": remoteAddr,
	}
	if password != "" {
		body["password"] = password
	}
	payload, status, err := u.post(ctx, "/api/v1/authentication/tokens/", body)
	if err != nil {
		return Result{}, err
	}
	return parseAuthPayload(payload, status)
}

func (u *UserClient) SelectMFA(ctx context.Context, username, mfaType string) error {
	payload, status, err := u.post(ctx, "/api/v1/authentication/mfa/select/", map[string]string{
		"username": username,
		"type":     mfaType,
	})
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("select mfa %s failed: %s", mfaType, payload.message())
	}
	return nil
}

func (u *UserClient) VerifyMFA(ctx context.Context, mfaType, code string) error {
	payload, status, err := u.post(ctx, "/api/v1/authentication/mfa/challenge/", map[string]string{
		"type": mfaType,
		"code": code,
	})
	if err != nil {
		return err
	}
	if status >= 400 || (payload.Error != "" && payload.Msg != "ok") {
		return fmt.Errorf("verify mfa failed: %s", payload.message())
	}
	return nil
}

func (u *UserClient) ConfirmStatus(ctx context.Context) (Result, error) {
	payload, status, err := u.do(ctx, http.MethodGet, "/api/v1/authentication/login-confirm-ticket/status/", nil)
	if err != nil {
		return Result{}, err
	}
	return parseAuthPayload(payload, status)
}

func (u *UserClient) CancelConfirm(ctx context.Context) error {
	_, _, err := u.do(ctx, http.MethodDelete, "/api/v1/authentication/login-confirm-ticket/status/", nil)
	return err
}

func (u *UserClient) post(ctx context.Context, path string, body any) (authPayload, int, error) {
	return u.do(ctx, http.MethodPost, path, body)
}

func (u *UserClient) do(ctx context.Context, method, path string, body any) (authPayload, int, error) {
	var reader io.Reader
	if body != nil {
		content, err := json.Marshal(body)
		if err != nil {
			return authPayload{}, 0, err
		}
		reader = bytes.NewReader(content)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.core.Host+path, reader)
	if err != nil {
		return authPayload{}, 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "JumpServer-SSHProxy")
	sign, err := ServiceSignature(u.core.Key, time.Now())
	if err != nil {
		return authPayload{}, 0, err
	}
	req.Header.Set("X-JMS-SVC", sign)
	resp, err := u.http.Do(req)
	if err != nil {
		return authPayload{}, 0, err
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return authPayload{}, resp.StatusCode, err
	}
	var payload authPayload
	if len(bytes.TrimSpace(content)) > 0 {
		if err = json.Unmarshal(content, &payload); err != nil {
			return authPayload{}, resp.StatusCode, fmt.Errorf("decode core response: %w", err)
		}
	}
	return payload, resp.StatusCode, nil
}

type authPayload struct {
	Token string   `json:"token"`
	Error string   `json:"error"`
	Msg   string   `json:"msg"`
	User  User     `json:"user"`
	Data  authData `json:"data"`
}

type authData struct {
	Choices []string `json:"choices"`
	URL     string   `json:"url"`
}

func (p authPayload) message() string {
	if p.Msg != "" {
		return p.Msg
	}
	if p.Error != "" {
		return p.Error
	}
	return "core request failed"
}

func parseAuthPayload(payload authPayload, status int) (Result, error) {
	if status >= 400 {
		return Result{}, fmt.Errorf("%s", payload.message())
	}
	switch payload.Error {
	case "":
		if payload.Token != "" || payload.Msg == "ok" {
			return Result{Phase: PhaseSuccess, User: payload.User, Message: payload.Msg}, nil
		}
		return Result{}, fmt.Errorf("unexpected core response")
	case "mfa_required":
		return Result{Phase: PhaseMFA, Choices: payload.Data.Choices, Message: payload.Msg}, nil
	case "login_confirm_wait":
		return Result{Phase: PhaseConfirm, Message: payload.Msg}, nil
	default:
		return Result{}, fmt.Errorf("%s", payload.message())
	}
}

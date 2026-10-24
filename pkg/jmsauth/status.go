package jmsauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jumpserver/jumpserver-sshproxy/pkg/logger"
)

const statusInterval = 20 * time.Second

const statusPath = "/api/v1/terminal/status/"

type statusReport struct {
	CPULoad       float64  `json:"cpu_load"`
	MemoryUsed    float64  `json:"memory_used"`
	DiskUsed      float64  `json:"disk_used"`
	SessionOnline int      `json:"session_online"`
	Sessions      []string `json:"sessions"`
}

func (c *Core) ReportStatus(ctx context.Context, sessions int) error {
	stat := readHostStat()
	if sessions < 0 {
		sessions = 0
	}
	body, err := json.Marshal(statusReport{
		CPULoad:       stat.CPULoad,
		MemoryUsed:    stat.MemoryUsed,
		DiskUsed:      stat.DiskUsed,
		SessionOnline: sessions,
		Sessions:      []string{},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Host+statusPath, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "JumpServer-SSHProxy")
	sign, err := ServiceSignature(c.Key, time.Now())
	if err != nil {
		return err
	}
	req.Header.Set("X-JMS-SVC", sign)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("report status: status %d: %s", resp.StatusCode, bytes.TrimSpace(content))
	}
	return nil
}

// KeepAlive reports component status until ctx is cancelled.
// online returns the in-memory session count for this process.
// The response task list is ignored. This component does not run terminal tasks.
func (c *Core) KeepAlive(ctx context.Context, online func() int) {
	if online == nil {
		online = func() int { return 0 }
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	logged := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			count := online()
			err := c.ReportStatus(ctx, count)
			if err != nil && ctx.Err() == nil {
				logger.Errorf("Report component status failed: %s", err)
			}
			if err == nil && !logged {
				logger.Infof("Component status reported session_online=%d", count)
				logged = true
			}
			if ctx.Err() != nil {
				return
			}
			timer.Reset(statusInterval)
		}
	}
}

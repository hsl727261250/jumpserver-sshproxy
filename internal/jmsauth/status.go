package jmsauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jumpserver/jumpserver-sshproxy/internal/logger"
)

const statusInterval = 20 * time.Second

const statusPath = "/api/v1/terminal/status/"

const taskKillSession = "kill_session"

type statusReport struct {
	CPULoad       float64  `json:"cpu_load"`
	MemoryUsed    float64  `json:"memory_used"`
	DiskUsed      float64  `json:"disk_used"`
	SessionOnline int      `json:"session_online"`
	Sessions      []string `json:"sessions"`
}

type terminalTask struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Args string `json:"args"`
}

func (c *Core) ReportStatus(ctx context.Context, online int, sessionIDs []string) ([]terminalTask, error) {
	stat := readHostStat()
	if online < 0 {
		online = 0
	}
	if sessionIDs == nil {
		sessionIDs = []string{}
	}
	content, _, err := c.signedJSON(ctx, http.MethodPost, statusPath, statusReport{
		CPULoad:       stat.CPULoad,
		MemoryUsed:    stat.MemoryUsed,
		DiskUsed:      stat.DiskUsed,
		SessionOnline: online,
		Sessions:      sessionIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("report status: %w", err)
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return nil, nil
	}
	var tasks []terminalTask
	if err = json.Unmarshal(content, &tasks); err != nil {
		logger.Errorf("Decode component tasks failed: %s", err)
		return nil, nil
	}
	return tasks, nil
}

// KeepAlive reports component status until ctx is cancelled.
// snapshot returns the session_online count and the Core session IDs to refresh.
// kill closes the SSH connection for a kill_session task. Other tasks are ignored.
func (c *Core) KeepAlive(ctx context.Context, snapshot func() (int, []string), kill func(sessionID string)) {
	if snapshot == nil {
		snapshot = func() (int, []string) { return 0, nil }
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	logged := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			online, ids := snapshot()
			tasks, err := c.ReportStatus(ctx, online, ids)
			if err != nil && ctx.Err() == nil {
				logger.Errorf("Report component status failed: %s", err)
			}
			if err == nil && !logged {
				logger.Infof("Component status reported session_online=%d", online)
				logged = true
			}
			if ctx.Err() != nil {
				return
			}
			c.runKillTasks(ctx, tasks, kill)
			if ctx.Err() != nil {
				return
			}
			timer.Reset(statusInterval)
		}
	}
}

func (c *Core) runKillTasks(ctx context.Context, tasks []terminalTask, kill func(sessionID string)) {
	for _, task := range tasks {
		if task.Name != taskKillSession || task.ID == "" {
			continue
		}
		if kill != nil && task.Args != "" {
			kill(task.Args)
		}
		if err := c.FinishTask(ctx, task.ID); err != nil && ctx.Err() == nil {
			logger.Errorf("Finish kill session task %s failed: %s", task.ID, err)
		}
	}
}

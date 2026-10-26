package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jumpserver/jumpserver-sshproxy/internal/config"
	"github.com/jumpserver/jumpserver-sshproxy/internal/jmsauth"
	"github.com/jumpserver/jumpserver-sshproxy/internal/logger"
	"github.com/jumpserver/jumpserver-sshproxy/internal/session"
	"github.com/jumpserver/jumpserver-sshproxy/internal/sshd"
)

type App struct {
	sshSrv *sshd.Server
}

func (a *App) Start() {
	go a.sshSrv.Start()
}

func (a *App) Stop() {
	a.sshSrv.Stop()
	logger.Infof("Quit SSHProxy")
}

func RunForever(confPath string) {
	config.Setup(confPath)
	logger.Initial()
	conf := config.GetConf()
	core := jmsauth.NewCore(conf.CoreHost, jmsauth.AccessKey{}, time.Duration(conf.HttpRequestTimeout)*time.Second)
	if err := core.EnsureAccessKey(conf.AccessKeyFilePath, conf.Name, conf.TerminalType, conf.BootstrapToken); err != nil {
		logger.Fatalf("Prepare access key failed: %s", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go core.KeepAlive(ctx, func() (int, []string) {
		if conf.AssetID == "" {
			return session.Online(), nil
		}
		ids := session.IDs()
		return len(ids), ids
	}, func(id string) { session.Terminate(id) })
	gracefulStop := make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	app := &App{sshSrv: sshd.NewSSHServer(core)}
	app.Start()
	<-gracefulStop
	cancel()
	app.Stop()
}

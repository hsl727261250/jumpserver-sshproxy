package app

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/jumpserver/jumpserver-sshproxy/pkg/config"
	"github.com/jumpserver/jumpserver-sshproxy/pkg/logger"
	"github.com/jumpserver/jumpserver-sshproxy/pkg/sshd"
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
	gracefulStop := make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	app := &App{sshSrv: sshd.NewSSHServer()}
	app.Start()
	<-gracefulStop
	app.Stop()
}

package app

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jumpserver/jumpserver-sshproxy/pkg/config"
	"github.com/jumpserver/jumpserver-sshproxy/pkg/jmsauth"
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
	conf := config.GetConf()
	core := jmsauth.NewCore(conf.CoreHost, jmsauth.AccessKey{}, time.Duration(conf.HttpRequestTimeout)*time.Second)
	if err := core.EnsureAccessKey(conf.AccessKeyFilePath, conf.Name, conf.TerminalType, conf.BootstrapToken); err != nil {
		logger.Fatalf("Prepare access key failed: %s", err)
	}
	gracefulStop := make(chan os.Signal, 1)
	signal.Notify(gracefulStop, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	app := &App{sshSrv: sshd.NewSSHServer(core)}
	app.Start()
	<-gracefulStop
	app.Stop()
}

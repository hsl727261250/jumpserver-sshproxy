package sshd

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/gliderlabs/ssh"

	"github.com/jumpserver/jumpserver-sshproxy/internal/auth"
	"github.com/jumpserver/jumpserver-sshproxy/internal/config"
	"github.com/jumpserver/jumpserver-sshproxy/internal/jmsauth"
	"github.com/jumpserver/jumpserver-sshproxy/internal/logger"
)

const sshChannelDirectTCPIP = "direct-tcpip"

type Server struct {
	Srv *ssh.Server
}

func NewSSHServer(core *jmsauth.Core) *Server {
	conf := config.GetConf()
	signer, err := LoadOrCreateHostKey(conf.HostKeyFilePath)
	if err != nil {
		logger.Fatalf("Load host key failed: %s", err)
	}
	authenticator := auth.NewAuthenticator(
		core,
		time.Duration(conf.MaxIdleMinutes)*time.Minute,
		time.Duration(conf.MaxSessionHours)*time.Hour,
		conf.AssetID,
	)
	srv := &ssh.Server{
		Addr:            net.JoinHostPort(config.ListenHost, config.ListenPort),
		Version:         "JumpServer-SSHProxy",
		HostSigners:     []ssh.Signer{signer},
		PasswordHandler: authenticator.Password,
		PtyCallback: func(ctx ssh.Context, pty ssh.Pty) bool {
			return false
		},
		LocalPortForwardingCallback: AllowLocalForward,
		Handler:                     sessionHandler,
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"session":             ssh.DefaultSessionHandler,
			sshChannelDirectTCPIP: DirectTCPIPHandler,
		},
	}
	return &Server{Srv: srv}
}

func (s *Server) Start() {
	logger.Infof("Start SSH server at %s", s.Srv.Addr)
	if err := s.Srv.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		logger.Errorf("SSH server stopped unexpectedly: %s", err)
	}
}

func (s *Server) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Srv.Shutdown(ctx); err != nil {
		logger.Errorf("Stop SSH server failed: %s", err)
	}
}

func sessionHandler(sess ssh.Session) {
	_, _ = io.WriteString(sess, "shell is disabled\n")
	_ = sess.Exit(1)
}

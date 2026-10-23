package sshd

import (
	"context"
	"errors"
	"io"
	"net"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	"github.com/jumpserver/jumpserver-sshproxy/pkg/config"
	"github.com/jumpserver/jumpserver-sshproxy/pkg/logger"
)

const sshChannelDirectTCPIP = "direct-tcpip"

var errAuthNotImplemented = errors.New("authentication is not implemented")

type Server struct {
	Srv *ssh.Server
}

func NewSSHServer() *Server {
	conf := config.GetConf()
	signer, err := LoadOrCreateHostKey(conf.HostKeyFilePath)
	if err != nil {
		logger.Fatalf("Load host key failed: %s", err)
	}
	srv := &ssh.Server{
		Addr:                       net.JoinHostPort(conf.BindHost, conf.SSHPort),
		Version:                    "JumpServer-SSHProxy",
		HostSigners:                []ssh.Signer{signer},
		PasswordHandler:            passwordAuth,
		KeyboardInteractiveHandler: keyboardInteractiveAuth,
		PtyCallback: func(ctx ssh.Context, pty ssh.Pty) bool {
			return false
		},
		// 认证和授权完成前拒绝转发。direct-tcpip 的连接由 gliderlabs 的处理器完成。
		LocalPortForwardingCallback: func(ctx ssh.Context, destinationHost string, destinationPort uint32) bool {
			return false
		},
		Handler: sessionHandler,
		ChannelHandlers: map[string]ssh.ChannelHandler{
			"session":             ssh.DefaultSessionHandler,
			sshChannelDirectTCPIP: ssh.DirectTCPIPHandler,
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

func passwordAuth(ctx ssh.Context, password string) error {
	logger.Infof("SSH conn[%s] password auth is not implemented, user=%s", ctx.SessionID(), ctx.User())
	return errAuthNotImplemented
}

func keyboardInteractiveAuth(ctx ssh.Context, challenger gossh.KeyboardInteractiveChallenge) error {
	logger.Infof("SSH conn[%s] keyboard-interactive auth is not implemented, user=%s", ctx.SessionID(), ctx.User())
	return errAuthNotImplemented
}

func sessionHandler(session ssh.Session) {
	_, _ = io.WriteString(session, "shell is disabled\n")
	_ = session.Exit(1)
}

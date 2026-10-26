package sshd

import (
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	"github.com/jumpserver/jumpserver-sshproxy/internal/config"
	"github.com/jumpserver/jumpserver-sshproxy/internal/logger"
	"github.com/jumpserver/jumpserver-sshproxy/internal/session"
)

const copyBufferSize = 32 * 1024

var copyPool = sync.Pool{
	New: func() any {
		buf := make([]byte, copyBufferSize)
		return &buf
	},
}

type localForwardChannelData struct {
	DestAddr string
	DestPort uint32

	OriginAddr string
	OriginPort uint32
}

func AllowLocalForward(ctx ssh.Context, destinationHost string, destinationPort uint32) bool {
	return session.From(ctx) != nil
}

func DirectTCPIPHandler(srv *ssh.Server, _ *gossh.ServerConn, newChan gossh.NewChannel, ctx ssh.Context) {
	data := localForwardChannelData{}
	if err := gossh.Unmarshal(newChan.ExtraData(), &data); err != nil {
		_ = newChan.Reject(gossh.ConnectionFailed, "error parsing forward data: "+err.Error())
		return
	}
	if srv.LocalPortForwardingCallback == nil || !srv.LocalPortForwardingCallback(ctx, data.DestAddr, data.DestPort) {
		_ = newChan.Reject(gossh.Prohibited, "port forwarding is disabled")
		return
	}
	dest := net.JoinHostPort(data.DestAddr, strconv.FormatUint(uint64(data.DestPort), 10))
	conf := config.GetConf()
	dialer := net.Dialer{
		Timeout:   time.Duration(conf.ForwardDialTimeout) * time.Second,
		KeepAlive: 30 * time.Second,
	}
	remote, err := dialer.DialContext(ctx, "tcp", dest)
	if err != nil {
		_ = newChan.Reject(gossh.ConnectionFailed, err.Error())
		return
	}
	channel, requests, err := newChan.Accept()
	if err != nil {
		_ = remote.Close()
		return
	}
	go gossh.DiscardRequests(requests)

	user := ""
	if current := session.From(ctx); current != nil {
		user = current.User
	}
	logger.Infof("SSH conn[%s] forward open user=%s dest=%s", ctx.SessionID(), user, dest)
	defer logger.Infof("SSH conn[%s] forward close user=%s dest=%s", ctx.SessionID(), user, dest)

	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = channel.Close()
			_ = remote.Close()
		})
	}
	touch := func() {
		if current := session.From(ctx); current != nil {
			current.Touch()
		}
	}
	go func() {
		_ = proxyCopy(channel, remote, touch)
		closeBoth()
	}()
	_ = proxyCopy(remote, channel, touch)
	closeBoth()
}

func proxyCopy(dst io.Writer, src io.Reader, touch func()) error {
	bufp := copyPool.Get().(*[]byte)
	defer copyPool.Put(bufp)
	buf := *bufp
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if touch != nil {
				touch()
			}
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

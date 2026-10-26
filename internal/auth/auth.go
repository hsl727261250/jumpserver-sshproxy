package auth

import (
	"context"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	gossh "golang.org/x/crypto/ssh"

	"github.com/jumpserver/jumpserver-sshproxy/internal/jmsauth"
	"github.com/jumpserver/jumpserver-sshproxy/internal/logger"
	"github.com/jumpserver/jumpserver-sshproxy/internal/session"
)

type clientKey struct{}
type phaseKey struct{}

type Authenticator struct {
	core    *jmsauth.Core
	idle    time.Duration
	max     time.Duration
	assetID string
}

func NewAuthenticator(core *jmsauth.Core, idle, max time.Duration, assetID string) *Authenticator {
	return &Authenticator{core: core, idle: idle, max: max, assetID: assetID}
}

func (a *Authenticator) Password(ctx ssh.Context, password string) error {
	client, err := a.core.UserClient()
	if err != nil {
		logger.Errorf("SSH conn[%s] create auth client failed: %s", ctx.SessionID(), err)
		return err
	}
	ctx.SetValue(clientKey{}, client)
	logger.Infof("SSH conn[%s] authenticating user %s", ctx.SessionID(), ctx.User())
	result, err := client.Authenticate(ctx, ctx.User(), password, remoteIP(ctx.RemoteAddr()))
	if err != nil {
		logger.Errorf("SSH conn[%s] auth failed user=%s err=%s", ctx.SessionID(), ctx.User(), err)
		return err
	}
	return a.finish(ctx, client, result)
}

func (a *Authenticator) Keyboard(ctx ssh.Context, challenger gossh.KeyboardInteractiveChallenge) error {
	client, _ := ctx.Value(clientKey{}).(*jmsauth.UserClient)
	phase, _ := ctx.Value(phaseKey{}).(jmsauth.Phase)
	if client == nil {
		return errAuthFailed
	}
	switch phase {
	case jmsauth.PhaseMFA:
		return a.mfa(ctx, client, challenger)
	case jmsauth.PhaseConfirm:
		return a.confirm(ctx, client, challenger)
	default:
		return errAuthFailed
	}
}

func (a *Authenticator) finish(ctx ssh.Context, client *jmsauth.UserClient, result jmsauth.Result) error {
	switch result.Phase {
	case jmsauth.PhaseSuccess:
		if result.User.Username == "" {
			result.User.Username = ctx.User()
		}
		s := session.New(result.User.Username)
		session.Store(ctx, s)
		watchCtx, stopWatch := context.WithCancel(ctx)
		coreID := a.openCoreSession(ctx, result.User, remoteIP(ctx.RemoteAddr()), s, func() {
			stopWatch()
			closeSSHConn(ctx)
		})
		logger.Infof("SSH conn[%s] session started user=%s", ctx.SessionID(), result.User.Username)
		session.Enter()
		go func() {
			defer session.Leave()
			defer stopWatch()
			if coreID != "" {
				defer func() {
					finishCtx, cancel := finishContext(a.core)
					defer cancel()
					if err := a.core.FinishSession(finishCtx, coreID); err != nil {
						logger.Errorf("SSH conn[%s] finish core session failed user=%s err=%s", ctx.SessionID(), s.User, err)
					}
					session.Untrack(coreID)
				}()
			}
			session.Watch(watchCtx, s, a.idle, a.max, func() io.Closer {
				conn, _ := ctx.Value(ssh.ContextKeyConn).(*gossh.ServerConn)
				if conn == nil {
					return nil
				}
				return conn
			})
		}()
		return nil
	case jmsauth.PhaseMFA:
		client.Choices = usableMFA(result.Choices)
		ctx.SetValue(clientKey{}, client)
		ctx.SetValue(phaseKey{}, jmsauth.PhaseMFA)
		return &ssh.PartialSuccessError{Next: ssh.ServerAuthCallbacks{
			KeyboardInteractiveCallback: a.Keyboard,
		}}
	case jmsauth.PhaseConfirm:
		ctx.SetValue(phaseKey{}, jmsauth.PhaseConfirm)
		return &ssh.PartialSuccessError{Next: ssh.ServerAuthCallbacks{
			KeyboardInteractiveCallback: a.Keyboard,
		}}
	default:
		return errAuthFailed
	}
}

func (a *Authenticator) mfa(ctx ssh.Context, client *jmsauth.UserClient, challenger gossh.KeyboardInteractiveChallenge) error {
	choices := client.Choices
	if len(choices) == 0 {
		_, _ = challenger(ctx.User(), "No MFA options are available. Configure MFA in JumpServer and try again.", []string{"Press enter to exit:"}, []bool{true})
		return errAuthFailed
	}
	selected := choices[0]
	if len(choices) > 1 {
		picked, err := selectMFA(ctx, challenger, choices)
		if err != nil {
			return err
		}
		selected = picked
	}
	if needsChallenge(selected) {
		if err := client.SelectMFA(ctx, ctx.User(), selected); err != nil {
			logger.Errorf("SSH conn[%s] select mfa failed user=%s err=%s", ctx.SessionID(), ctx.User(), err)
			return errAuthFailed
		}
	}
	code, err := ask(ctx, challenger, "Please enter the MFA code.", strings.ToUpper(selected)+" code:")
	if err != nil {
		return err
	}
	if code == "" {
		return errAuthFailed
	}
	if err = client.VerifyMFA(ctx, selected, code); err != nil {
		logger.Errorf("SSH conn[%s] mfa failed user=%s err=%s", ctx.SessionID(), ctx.User(), err)
		return errAuthFailed
	}
	result, err := client.Authenticate(ctx, ctx.User(), "", remoteIP(ctx.RemoteAddr()))
	if err != nil {
		logger.Errorf("SSH conn[%s] auth failed user=%s err=%s", ctx.SessionID(), ctx.User(), err)
		return errAuthFailed
	}
	return a.finish(ctx, client, result)
}

func (a *Authenticator) confirm(ctx ssh.Context, client *jmsauth.UserClient, challenger gossh.KeyboardInteractiveChallenge) error {
	answer, err := ask(ctx, challenger, "Login requires administrator confirmation.", "Wait for confirmation? [Y/n]:")
	if err != nil {
		_ = client.CancelConfirm(ctx)
		return err
	}
	switch strings.ToLower(answer) {
	case "", "y", "yes":
	case "n", "no":
		_ = client.CancelConfirm(ctx)
		return errAuthFailed
	default:
		return errAuthFailed
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			cancelCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = client.CancelConfirm(cancelCtx)
			cancel()
			return errAuthFailed
		case <-ticker.C:
			result, err := client.ConfirmStatus(ctx)
			if err != nil {
				logger.Errorf("SSH conn[%s] login confirm failed user=%s err=%s", ctx.SessionID(), ctx.User(), err)
				return errAuthFailed
			}
			if result.Phase == jmsauth.PhaseConfirm {
				continue
			}
			if result.Phase != jmsauth.PhaseSuccess {
				return errAuthFailed
			}
			result, err = client.Authenticate(ctx, ctx.User(), "", remoteIP(ctx.RemoteAddr()))
			if err != nil {
				return errAuthFailed
			}
			return a.finish(ctx, client, result)
		}
	}
}

func selectMFA(ctx ssh.Context, challenger gossh.KeyboardInteractiveChallenge, choices []string) (string, error) {
	var lines []string
	for i, choice := range choices {
		lines = append(lines, strconv.Itoa(i+1)+") "+choice)
	}
	question := strings.Join(lines, "\n") + "\nEnter the number:"
	for i := 0; i < 3; i++ {
		answer, err := ask(ctx, challenger, "Please select an MFA option.", question)
		if err != nil {
			return "", err
		}
		n, _ := strconv.Atoi(answer)
		if n >= 1 && n <= len(choices) {
			return choices[n-1], nil
		}
	}
	return "", errAuthFailed
}

func ask(ctx ssh.Context, challenger gossh.KeyboardInteractiveChallenge, instruction, question string) (string, error) {
	answers, err := challenger(ctx.User(), instruction, []string{question}, []bool{true})
	if err != nil {
		return "", err
	}
	if len(answers) == 0 {
		return "", nil
	}
	return strings.TrimSpace(answers[0]), nil
}

func usableMFA(choices []string) []string {
	usable := make([]string, 0, len(choices))
	for _, choice := range choices {
		if strings.EqualFold(choice, "face") {
			continue
		}
		usable = append(usable, choice)
	}
	return usable
}

func needsChallenge(mfaType string) bool {
	switch strings.ToLower(mfaType) {
	case "sms", "email":
		return true
	default:
		return false
	}
}

func (a *Authenticator) openCoreSession(ctx ssh.Context, user jmsauth.User, remoteAddr string, s *session.Session, close func()) string {
	if a.assetID == "" {
		return ""
	}
	if user.ID == "" {
		logger.Errorf("SSH conn[%s] skip core session user=%s err=user id is empty", sessionID(ctx), user.Username)
		return ""
	}
	id, err := a.core.CreateSession(ctx, jmsauth.SessionSpec{
		User:       user.Username,
		UserID:     user.ID,
		AssetID:    a.assetID,
		RemoteAddr: remoteAddr,
	})
	if err != nil {
		logger.Errorf("SSH conn[%s] create core session failed user=%s err=%s", sessionID(ctx), user.Username, err)
		return ""
	}
	session.Track(id, s, close)
	logger.Infof("SSH conn[%s] core session %s user=%s", ctx.SessionID(), id, user.Username)
	return id
}

func sessionID(ctx ssh.Context) string {
	if ctx == nil {
		return ""
	}
	return ctx.SessionID()
}

func closeSSHConn(ctx ssh.Context) {
	conn, _ := ctx.Value(ssh.ContextKeyConn).(*gossh.ServerConn)
	if conn != nil {
		_ = conn.Close()
	}
}

func finishContext(core *jmsauth.Core) (context.Context, context.CancelFunc) {
	timeout := core.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return context.WithTimeout(context.Background(), timeout)
}

func remoteIP(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

var errAuthFailed = authError("authentication failed")

type authError string

func (e authError) Error() string { return string(e) }

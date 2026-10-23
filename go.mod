module github.com/jumpserver/jumpserver-sshproxy

go 1.27.3

require (
	github.com/gliderlabs/ssh v0.3.8
	golang.org/x/crypto v0.52.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be // indirect
	golang.org/x/sys v0.47.0 // indirect
)

replace github.com/gliderlabs/ssh => github.com/jumpserver-dev/ssh v0.3.11

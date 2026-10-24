#!/bin/sh
set -eu

cd "$(dirname "$0")"

export PATH="/usr/local/go/bin:${PATH}"

if [ -f .env ]; then
    set -a
    # shellcheck disable=SC1091
    . ./.env
    set +a
elif [ -f example.env ]; then
    set -a
    # shellcheck disable=SC1091
    . ./example.env
    set +a
fi

image="${SSHPROXY_IMAGE:-jumpserver-sshproxy}"
version="${SSHPROXY_VERSION:-latest}"

HTTP_PROXY=socks5h://127.0.0.1:1080 \
HTTPS_PROXY=socks5h://127.0.0.1:1080 \
ALL_PROXY=socks5h://127.0.0.1:1080 \
GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}" \
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags "-s -w" -o sshproxy ./cmd/sshproxy

docker build -t "${image}:${version}" .

#!/bin/sh
set -eu

cd "$(dirname "$0")"
root="$(CDPATH= cd .. && pwd)"

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

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -ldflags "-s -w" -o "${root}/sshproxy" "${root}/cmd/sshproxy"

docker build -f Dockerfile -t "${image}:${version}" "${root}"

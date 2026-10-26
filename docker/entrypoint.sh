#!/bin/sh
set -eu

if [ -n "${CORE_HOST:-}" ]; then
    echo "wait for jms_core ${CORE_HOST} ready"
    until curl -fsS "${CORE_HOST%/}/api/health/" >/dev/null; do
        sleep 2
    done
fi

cd /opt/sshproxy
exec /opt/sshproxy/sshproxy

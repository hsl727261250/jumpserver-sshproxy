FROM debian:trixie-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /opt/sshproxy
COPY sshproxy /opt/sshproxy/sshproxy
COPY entrypoint.sh /opt/sshproxy/entrypoint.sh
COPY config_example.yml /opt/sshproxy/config_example.yml
RUN chmod 755 /opt/sshproxy/entrypoint.sh /opt/sshproxy/sshproxy

VOLUME /opt/sshproxy/data
EXPOSE 1024
ENTRYPOINT ["/opt/sshproxy/entrypoint.sh"]
STOPSIGNAL SIGQUIT

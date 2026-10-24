FROM debian:trixie-slim

# trixie-slim 使用 DEB822 源。阿里云同时镜像 /debian 与 /debian-security，
# 只替换主机名即可。安装 ca-certificates 之前使用 HTTP。
RUN sed -i 's|deb.debian.org|mirrors.aliyun.com|g' /etc/apt/sources.list.d/debian.sources \
    && apt-get update \
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

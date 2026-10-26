# JumpServer SSHProxy

SSHProxy 是一个 SSH 隧道组件。它监听 `1024` 端口，使用 JumpServer 的用户名和密码登录。账号开启了 MFA 时，会继续通过 SSH 的 keyboard-interactive 收取 OTP、短信或邮件验证码。登录复核开启时，会等待管理员确认。

认证成功后，客户端可以用这条 SSH 连接转发任意 TCP 流量，例如 `ssh -L` 和 `ssh -D`。组件不提供交互式 shell，也不提供 SFTP。

启动时用 `BOOTSTRAP_TOKEN` 向 JumpServer Core 注册，并保存 AccessKey。之后的认证请求和状态上报都带服务账号签名，Core 才能记录真实的客户端 IP。注册类型默认是 `koko`，因为 Core 目前没有单独的 sshproxy 类型。进程每 20 秒向 Core 上报 CPU 负载、内存和磁盘。配置了 `ASSET_ID` 后，认证成功会创建一条 `proxy` 隧道会话，心跳在 `sessions` 里带上这条会话的 ID，连接断开时把会话标为结束。管理员在审计页终止会话后，组件在下一次心跳收到 `kill_session` 并关闭这条 SSH 连接。未配置 `ASSET_ID` 时只上报本进程里的连接数，`sessions` 为空，不会写成 Core 会话。除 `kill_session` 以外的终端任务会被忽略。会话 ID 只保存在内存里，进程退出后丢失。

## 打包

在 `docker` 目录执行：

```bash
cd docker
./build.sh
```

脚本回到仓库根目录编译 Linux 二进制 `sshproxy`，再以仓库根目录为构建上下文执行 `docker build`。标签来自 `docker/.env` 里的 `SSHPROXY_IMAGE` 和 `SSHPROXY_VERSION`；还没有 `.env` 时使用 `docker/example.env`。镜像内不安装 Go，也不在镜像里编译。脚本不登录仓库，也不推送镜像。

`go build` 使用本机已有的 Go 模块缓存。`docker build` 使用本地的 `debian:trixie-slim`，构建时把 apt 源换成阿里云开源镜像站，再安装 `ca-certificates` 和 `curl`。

运行镜像基于 `debian:trixie-slim`。`docker/Dockerfile` 只复制仓库根目录的 `sshproxy`、`docker/entrypoint.sh` 和 `config_example.yml`。工作目录是 `/opt/sshproxy`，监听端口是 `1024`，数据目录是 `/opt/sshproxy/data`。根目录的 `sshproxy` 已被 `.gitignore` 忽略。

## 部署前准备

JumpServer Core 需要允许组件注册，`BOOTSTRAP_TOKEN` 必须和 Core 配置里的值一致。Core 的健康检查地址是 `/api/health/`。容器设置了 `CORE_HOST` 后，入口脚本会一直等待这个地址返回成功，然后再启动进程。

复制两个模板后再改配置。`docker/.env` 和仓库根目录的 `config.yml` 不要提交到仓库：

```bash
cp docker/example.env docker/.env
cp config_example.yml config.yml
```

至少修改这几项：

- `SSHPROXY_IMAGE`、`SSHPROXY_VERSION`：本地构建时保持 `jumpserver-sshproxy` 和 `latest`。换成仓库镜像时写成仓库地址和版本，例如 `registry.example.com/jumpserver-sshproxy` 和 `v1.0.0`。
- `SSHPROXY_NETWORK`：Core 已经加入的 Docker 网络名称。网络不存在时先执行 `docker network create jumpserver`。
- `CORE_HOST`：容器在这个网络里访问 Core 的地址。
- `BOOTSTRAP_TOKEN`：和 Core 配置一致。AccessKey 写入宿主机数据目录之后可以留空。
- `SSHPROXY_DATA_DIR`：宿主机数据目录，默认 `/opt/jumpserver-data/koko/data`，会挂到容器内 `/opt/sshproxy/data`。目录需要事先存在。

## 用 Compose 启动

先在 `docker` 目录执行 `./build.sh`。脚本按 `docker/.env` 中的镜像名和版本打本地标签。`docker/docker-compose.yml` 只引用同目录的 `.env`，并把仓库根目录的 `config.yml` 只读挂进容器。启动时不重新构建。

```bash
cd docker
docker compose up -d
docker compose logs -f sshproxy
```

正常时会先打印等待 Core，然后出现 `Start SSH server at 0.0.0.0:1024`。注册失败时容器会退出。常见原因是 Token 不一致、Core 还没有允许注册，或者 `CORE_HOST` 从 `SSHPROXY_NETWORK` 里访问不到。

## 配置

传给进程的环境变量只有 `CORE_HOST` 和 `BOOTSTRAP_TOKEN`。进程先读 `config.yml`，再用这两个环境变量覆盖文件中的同名项。SSH 固定监听 `0.0.0.0:1024`。

`.env` 里还有 Compose 使用的编排项，它们不会传进进程：

| 变量 | 模板值 | 作用 |
| --- | --- | --- |
| `SSHPROXY_IMAGE` | `jumpserver-sshproxy` | 镜像名或仓库地址 |
| `SSHPROXY_VERSION` | `latest` | 镜像版本 |
| `SSHPROXY_PORT` | `1024` | 宿主机映射端口，容器内固定监听 1024 |
| `SSHPROXY_NETWORK` | `jumpserver` | Core 所在的 Docker 网络 |
| `SSHPROXY_DATA_DIR` | `/opt/jumpserver-data/koko/data` | 宿主机数据目录，挂到容器内 `/opt/sshproxy/data` |
| `CORE_HOST` | `http://core:8080` | JumpServer Core 地址 |
| `BOOTSTRAP_TOKEN` | 空 | 首次注册使用的预共享密钥 |

`config.yml` 由 Compose 只读挂载到 `/opt/sshproxy/config.yml`。可以修改：

| 配置 | 模板值 | 作用 |
| --- | --- | --- |
| `NAME` | 不填 | 向 Core 注册时使用的名称，不填则按主机名生成 |
| `LOG_LEVEL` | `INFO` | `DEBUG`、`INFO`、`WARN`、`ERROR` |
| `HTTP_REQUEST_TIMEOUT` | `30` | 调用 Core 的超时时间，单位秒 |
| `TERMINAL_TYPE` | `koko` | 注册到 Core 的终端类型 |
| `FORWARD_DIAL_TIMEOUT` | `15` | 转发目标的拨号超时，单位秒 |
| `MAX_IDLE_MINUTES` | `30` | 无转发流量时断开，`0` 表示不限制 |
| `MAX_SESSION_HOURS` | `24` | 单条 SSH 连接的最长时间，`0` 表示不限制 |
| `ASSET_ID` | 不填 | 创建 Core 会话使用的资产 ID。资产需在组件服务账号的默认组织中。不填则不创建会话 |

## 客户端使用

把 `proxy.example.com` 换成 SSHProxy 的地址，端口换成 `.env` 里的 `SSHPROXY_PORT`，`alice` 换成 JumpServer 用户名。容器内固定监听 1024。

转发单个 TCP 端口：

```bash
ssh -p 1024 -N -L 127.0.0.1:15432:10.0.0.8:5432 alice@proxy.example.com
```

作为本地 SOCKS 代理：

```bash
ssh -p 1024 -N -D 127.0.0.1:1080 alice@proxy.example.com
```

账号开启 OTP 时，SSH 客户端会继续提示输入验证码。开启了登录复核时，按提示等待管理员确认。

## 数据目录

`SSHPROXY_DATA_DIR` 指向宿主机目录，Compose 把它挂到容器内的 `/opt/sshproxy/data`。默认是 `/opt/jumpserver-data/koko/data`。启动前确认这个目录存在：

```bash
mkdir -p /opt/jumpserver-data/koko/data
```

目录里会有：

- `keys/ssh_host_ed25519_key`：SSH 主机密钥。丢失后客户端会提示主机指纹变化。
- `keys/.access_key`：向 Core 注册得到的 AccessKey。存在时不会重复注册。
- `logs/sshproxy.log`：运行日志。

升级时重新打包本地镜像，再用 Compose 重建容器。这个宿主机目录会保留：

```bash
cd docker
./build.sh
docker compose up -d
```

只有在确认要重新注册时才删除其中的 `keys/.access_key`。删除后必须重新提供有效的 `BOOTSTRAP_TOKEN`。

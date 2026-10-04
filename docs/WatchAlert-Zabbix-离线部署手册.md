# WatchAlert Zabbix 告警中心离线部署手册

本文保留离线部署包的操作流程；所有地址均为示例，实际配置和 Secret 仅在本地保存。此源码仓库不包含文中离线镜像包。

## 1. 文档信息

| 项目 | 内容 |
| --- | --- |
| 适用项目 | WatchAlert Zabbix 告警中心改造版 |
| 适用部署包 | `WatchAlert-Linux-Offline-20260907-v9.zip` |
| 部署方式 | Docker Compose 离线镜像部署 |
| 适用架构 | Linux x86_64 / amd64 |
| Web 端口 | TCP 8088 |
| Zabbix 地址 | `http://192.0.2.82:8080` |
| 文档日期 | 2026-09-15 |

部署包已经包含 WatchAlert 前端、WatchAlert 后端、MySQL 8.0 和 Redis 7.4 镜像。生产服务器不需要安装 Go、Node.js 或 npm，也不需要现场编译或联网拉取镜像。

## 2. 系统架构

```text
浏览器
  |
  | TCP/8088
  v
w8t-web (Nginx + WatchAlert Web)
  |
  | 容器网络 TCP/9001
  v
w8t-service (WatchAlert Backend)
  |                         |
  | TCP/3306                | TCP/6379
  v                         v
w8t-mysql                 w8t-redis
  |
  | 持久化卷
  v
watchalert_v5_mysql_data

w8t-service --HTTP/HTTPS--> Zabbix /api_jsonrpc.php
w8t-service --HTTPS/443----> 飞书机器人 Webhook
```

容器说明：

| 容器 | 用途 | 宿主机端口 | 正常状态 |
| --- | --- | --- | --- |
| `w8t-web` | Web 前端和 API 反向代理 | `8088:80` | `healthy` |
| `w8t-service` | 告警处理、Zabbix 同步和通知 | 不对外暴露 | `healthy` |
| `w8t-mysql` | 业务数据库 | 不对外暴露 | `healthy` |
| `w8t-redis` | 缓存和内部消息 | 不对外暴露 | `healthy` |
| `w8t-init-mysql` | 首次初始化或补充数据库表 | 不对外暴露 | `Exited (0)` |

`w8t-init-mysql` 执行完成后显示 `Exited (0)` 是正常结果，不是故障。

## 3. 部署前准备

### 3.1 服务器要求

- Linux x86_64/amd64。
- Docker Engine 24 或更高版本。
- Docker Compose v2，命令形式必须是 `docker compose`。
- 建议至少 4 核 CPU、8 GB 内存、50 GB 可用磁盘。
- 服务器能够访问 Zabbix `192.0.2.82:8080`。
- 如需飞书通知，服务器能够访问飞书机器人 HTTPS 地址的 TCP 443 端口。
- 服务器时间和时区正确，建议启用 NTP。

检查命令：

```bash
uname -m
docker version
docker compose version
df -h /opt
timedatectl
```

`uname -m` 应返回 `x86_64`。ARM64 服务器不能直接使用本部署包。

### 3.2 网络和防火墙

建议只对可信管理网段开放：

| 方向 | 协议/端口 | 用途 |
| --- | --- | --- |
| 客户端到 WatchAlert | TCP 8088 | Web 访问 |
| WatchAlert 到 Zabbix | TCP 8080 | Zabbix JSON-RPC API |
| WatchAlert 到飞书 | TCP 443 | 飞书机器人通知 |

MySQL 3306、Redis 6379 和后端 9001 均只在 Docker 网络内部使用，不需要对宿主机或外网开放。

### 3.3 检查 Docker 网段冲突

本文 Zabbix 地址 `192.0.2.82` 是文档示例，请替换成实际地址。如果实际地址与 Docker 网段重叠，可能导致容器无法访问 Zabbix。

```bash
ip route get 192.0.2.82
docker network ls
docker network inspect bridge
```

本项目固定使用 `10.254.253.0/24`，网络名为 `watchalert_w8t`。如果该网段与现网、VPN 或云网络冲突，部署前修改 `docker-compose.yml` 中的子网。

## 4. 部署包校验和上传

部署文件：

```text
WatchAlert-Linux-Offline-20260907-v9.zip
WatchAlert-Linux-Offline-20260907-v9.zip.sha256
```

v9 压缩包 SHA256：

```text
8ef644f5a975b7e691c35c29aa1bbbcf7eb02f717c979e229c814acb31b52f27
```

将两个文件上传到服务器，例如 `/opt`：

```bash
cd /opt
sha256sum -c WatchAlert-Linux-Offline-20260907-v9.zip.sha256
```

应显示：

```text
WatchAlert-Linux-Offline-20260907-v9.zip: OK
```

如果校验文件由 Windows 工具重新保存并出现文件名不存在，可先清理 CRLF：

```bash
sed -i 's/\r$//' WatchAlert-Linux-Offline-20260907-v9.zip.sha256
sha256sum -c WatchAlert-Linux-Offline-20260907-v9.zip.sha256
```

## 5. 首次部署

### 5.1 解压

```bash
cd /opt
unzip WatchAlert-Linux-Offline-20260907-v9.zip
cd WatchAlert-Linux-Offline-20260907
chmod +x scripts/*.sh
```

主要目录：

| 路径 | 用途 |
| --- | --- |
| `docker-compose.yml` | 容器编排配置 |
| `images/` | 四个离线 Docker 镜像和校验文件 |
| `config/config.yaml.example` | 后端配置模板 |
| `secrets/` | MySQL Docker Secret |
| `scripts/install.sh` | 校验、加载镜像和启动服务 |
| `scripts/verify.sh` | 部署后健康检查 |
| `backend/deploy/helmchart/files/sql/` | MySQL 初始化 SQL |

### 5.2 检查敏感文件权限

部署包已包含本次私有部署使用的随机 MySQL Secret。不要把 `secrets/` 上传到 Git、网盘公开目录或聊天中。

```bash
chmod 600 secrets/mysql_root_password.txt
chmod 600 secrets/mysql_password.txt
ls -l secrets/
```

### 5.3 执行安装

```bash
./scripts/install.sh
```

脚本会依次执行：

1. 检查 Docker、Docker Compose、CPU 架构和必要文件。
2. 校验 `images/watchalert-images-linux-amd64.tar`。
3. 使用 `docker load` 加载四个镜像。
4. 检查 Compose 配置。
5. 使用 `--pull never --no-build` 启动容器。

服务器不会编译源码，也不会下载镜像。

### 5.4 查看启动状态

```bash
docker compose ps -a
```

预期结果：

- `w8t-mysql`：`healthy`
- `w8t-redis`：`healthy`
- `w8t-service`：`healthy`
- `w8t-web`：`healthy`
- `w8t-init-mysql`：`Exited (0)`

首次初始化 MySQL 通常需要几十秒。不要在 MySQL 尚未健康时反复执行安装脚本。

## 6. 部署验证

### 6.1 自动验证

```bash
./scripts/verify.sh
```

正常输出应包括：

- 后端 `/hello` 返回成功。
- 前端 `/healthz` 返回 `ok`。
- Zabbix `apiinfo.version` 返回版本号。

验证脚本不会读取或输出 Zabbix API Token。

### 6.2 手动验证

```bash
# 后端健康检查
docker compose exec -T w8t-service \
  wget -qO- http://127.0.0.1:9001/hello

# 前端健康检查
docker compose exec -T w8t-web \
  wget -qO- http://127.0.0.1/healthz

# 从后端容器检查 Zabbix API 路由
docker compose exec -T w8t-service wget -qO- \
  --header='Content-Type: application/json-rpc' \
  --post-data='{"jsonrpc":"2.0","method":"apiinfo.version","params":{},"id":1}' \
  'http://192.0.2.82:8080/api_jsonrpc.php'
```

浏览器访问：

```text
http://服务器IP:8088/
```

新数据库没有管理员时，按照登录页面提示初始化管理员密码。初始化完成后立即使用强密码，并限制 8088 端口的访问来源。

## 7. WatchAlert 业务初始化

推荐按照以下顺序配置：

1. 初始化 WatchAlert 管理员。
2. 创建通知对象，例如飞书机器人。
3. 创建故障中心并记录故障中心 ID。
4. 在 Zabbix 创建只读 API 用户和 API Token。
5. 在 WatchAlert 创建 Zabbix 数据源。
6. 测试数据源连接并保存。
7. 创建 Zabbix 查询规则和通知路由。
8. 触发一条测试告警，再恢复该告警，验证完整生命周期。

### 7.1 Zabbix API 权限要求

- 使用独立的只读 API 用户。
- 只授予需要接入的主机组读取权限。
- 不授予确认、关闭、维护期或配置写入权限。
- API 地址为 Zabbix 前端地址或完整的 `/api_jsonrpc.php` 地址。

本项目通过 Zabbix JSON-RPC API 轮询问题、事件、触发器和主机信息，不直接读取 Zabbix 数据库，也不会回写或关闭 Zabbix 告警。

### 7.2 推荐的 Zabbix 数据源参数

| 参数 | 建议值 | 说明 |
| --- | --- | --- |
| URL | `http://192.0.2.82:8080` | 也可填写完整 API 地址 |
| Poll Interval | `15` 秒 | 当前问题轮询间隔 |
| Timeout | `10` 秒 | 单次 API 超时 |
| Initial Lookback | `3600` 秒 | 首次启动回看范围 |
| Overlap | `300` 秒 | 重叠对账窗口 |
| TLS Skip Verify | `false` | HTTPS 正式环境不要跳过证书校验 |

严重性建议映射：

| Zabbix 严重性 | WatchAlert |
| --- | --- |
| Disaster（5） | P0 |
| High（4） | P1 |
| Average（3） | P2 |
| Warning（2） | P2 |
| Information（1） | P2 |
| Not classified（0） | P2 |

### 7.3 首次联调验收

至少完成以下测试：

1. 产生一条新的 Zabbix Problem。
2. 确认 WatchAlert 当前告警列表出现该事件。
3. 确认设备、触发器、严重性和告警正文完整。
4. 确认飞书告警卡片标题和正文均有内容。
5. 在 Zabbix 恢复问题。
6. 确认 WatchAlert 将同一指纹事件关闭并生成恢复通知。
7. 确认告警历史中只有一条完整生命周期记录，不产生重复事件。

旧的空白飞书卡片不会被自动补写，必须使用新触发的告警验证 v9 修复。

## 8. 日常运维

```bash
# 查看所有容器，包括已退出的初始化容器
docker compose ps -a

# 实时查看后端日志
docker compose logs -f --tail=200 w8t-service

# 查看前端日志
docker compose logs -f --tail=100 w8t-web

# 查看数据库日志
docker compose logs --tail=200 w8t-mysql

# 重启前后端，不重启数据库
docker compose restart w8t-service w8t-web

# 使用已加载镜像重新创建前后端
docker compose up -d --pull never --no-build --force-recreate \
  w8t-service w8t-web
```

查看 Zabbix 同步日志：

```bash
docker compose logs --since=30m w8t-service | \
  grep -E 'Zabbix 同步已启动|Zabbix 同步完成|Zabbix.*失败'
```

## 9. 数据备份与恢复

### 9.1 MySQL 逻辑备份

```bash
cd /opt/WatchAlert-Linux-Offline-20260907
mkdir -p backup
docker compose exec -T w8t-mysql sh -c \
  'MYSQL_PWD="$(cat /run/secrets/mysql_root_password)" mysqldump -uroot --single-transaction --routines --triggers watchalert' \
  > "backup/watchalert-$(date +%F-%H%M%S).sql"
```

检查备份：

```bash
ls -lh backup/
tail -n 5 backup/watchalert-*.sql
```

### 9.2 恢复数据库

恢复会覆盖或合并当前库内对象，应在维护窗口执行，并先额外备份当前数据库。

```bash
docker compose stop w8t-service w8t-web
docker compose exec -T w8t-mysql sh -c \
  'MYSQL_PWD="$(cat /run/secrets/mysql_root_password)" mysql -uroot watchalert' \
  < backup/需要恢复的文件.sql
docker compose start w8t-service w8t-web
./scripts/verify.sh
```

### 9.3 需要长期保存的内容

- MySQL 备份文件。
- `secrets/mysql_root_password.txt`。
- `secrets/mysql_password.txt`。
- 当前使用的 `docker-compose.yml`。
- 当前离线部署包及其 SHA256 文件。

不要只备份容器。容器可以重新创建，业务数据位于 Docker 卷 `watchalert_v5_mysql_data`。

## 10. 升级流程

升级前必须备份数据库，并保留现有 Secret。新包中的随机 Secret 不能直接覆盖旧 Secret，否则已有 MySQL 数据卷仍使用旧密码，后端将无法连接数据库。

```bash
# 1. 在旧目录备份数据库
cd /opt/WatchAlert-Linux-Offline-20260907
mkdir -p backup
docker compose exec -T w8t-mysql sh -c \
  'MYSQL_PWD="$(cat /run/secrets/mysql_root_password)" mysqldump -uroot --single-transaction watchalert' \
  > "backup/watchalert-before-upgrade-$(date +%F-%H%M%S).sql"

# 2. 解压新版本到独立目录
cd /opt
unzip WatchAlert-新版本.zip

# 3. 将旧 Secret 复制到新目录
cp /opt/WatchAlert-Linux-Offline-20260907/secrets/mysql_root_password.txt \
   /opt/WatchAlert-新版本目录/secrets/mysql_root_password.txt
cp /opt/WatchAlert-Linux-Offline-20260907/secrets/mysql_password.txt \
   /opt/WatchAlert-新版本目录/secrets/mysql_password.txt
chmod 600 /opt/WatchAlert-新版本目录/secrets/*.txt

# 4. 加载新镜像并重新创建服务
cd /opt/WatchAlert-新版本目录
chmod +x scripts/*.sh
./scripts/install.sh
./scripts/verify.sh
```

Compose 项目名、容器名和数据卷名保持不变，因此正常升级不会删除 MySQL 或 Redis 数据。

## 11. 回滚流程

### 11.1 仅回滚应用镜像

如果数据库结构没有发生不兼容变更，可重新进入旧版本目录加载旧镜像并重建服务：

```bash
cd /opt/WatchAlert-Linux-Offline-旧版本
docker load -i images/watchalert-images-linux-amd64.tar
docker compose up -d --pull never --no-build --force-recreate \
  w8t-service w8t-web
./scripts/verify.sh
```

### 11.2 数据库也需要回滚

如果升级包含数据库结构变更，不能只回滚镜像。停止前后端后，恢复升级前的 MySQL 备份，再启动旧版本并验证。

## 12. 常见故障处理

### 12.1 `sha256sum: listed file could not be read`

原因通常是校验文件使用 Windows CRLF 换行。

```bash
sed -i 's/\r$//' images/watchalert-images-linux-amd64.tar.sha256
./scripts/install.sh
```

v9 安装脚本已经兼容 LF 和 CRLF，此命令主要用于修复旧包或被其他工具重新保存的校验文件。

### 12.2 `pull access denied` 或尝试访问 Docker Hub

先确认镜像已加载：

```bash
docker image ls | grep -E 'watchalert|mysql|redis'
docker load -i images/watchalert-images-linux-amd64.tar
docker compose up -d --pull never --no-build
```

### 12.3 后端无法连接 MySQL

```bash
docker compose ps -a
docker compose logs --tail=200 w8t-mysql
docker compose logs --tail=200 w8t-service
```

如果是升级后发生，优先确认新目录中的两个 Secret 是否沿用了旧部署值。不要在聊天或工单中粘贴 Secret 内容。

### 12.4 容器无法访问 Zabbix

```bash
ip route get 192.0.2.82
docker network inspect watchalert_w8t
docker compose exec -T w8t-service wget -S -O- \
  'http://192.0.2.82:8080/api_jsonrpc.php'
```

重点检查：

- Docker 网段与 `192.0.2.0/24` 是否重叠。
- Zabbix 服务是否监听 8080。
- 主机和中间防火墙是否允许访问。
- 返回路由是否正确。

### 12.5 Web 页面可以打开，但 API 请求失败

```bash
docker compose ps
docker compose logs --tail=200 w8t-service
docker compose logs --tail=100 w8t-web
docker compose exec -T w8t-web wget -S -O- http://w8t-service:9001/hello
```

### 12.6 飞书收到卡片但正文为空

v9 已修复 Zabbix 告警正文包含换行、双引号或反斜杠时破坏飞书 JSON 卡片的问题。

检查当前后端是否使用 v9 镜像并重新创建：

```bash
docker image inspect watchalert-backend:zabbix-api-polling \
  --format '{{.Id}} {{.Os}}/{{.Architecture}}'
docker compose up -d --pull never --no-build --force-recreate w8t-service
docker compose logs -f --since=2m w8t-service
```

然后触发一条新的 Zabbix 测试告警。已经发送的空卡片不会自动更新。

## 13. 安全建议

- Zabbix API Token 只授予读取权限，并定期轮换。
- 飞书 Webhook、Zabbix Token 和 MySQL Secret 不写入文档、日志截图或工单。
- 8088 只允许可信管理网段访问。
- 正式环境在前方部署 HTTPS 反向代理。
- 定期备份 MySQL，并将备份复制到服务器之外。
- 不把 `secrets/`、数据库备份或包含真实配置的压缩包提交到代码仓库。
- 不对外暴露 MySQL、Redis 和后端 9001 端口。

## 14. 停止和卸载

仅停止服务并保留数据：

```bash
docker compose down
```

重新启动：

```bash
docker compose up -d --pull never --no-build
```

以下命令会永久删除 MySQL 和 Redis 数据卷，仅允许在确认已有可用备份且明确需要彻底卸载时执行：

```bash
docker compose down -v
```

## 15. 最终验收清单

- [ ] 压缩包 SHA256 校验通过。
- [ ] 服务器架构为 x86_64/amd64。
- [ ] Docker 和 Docker Compose v2 正常。
- [ ] Docker 网段与 Zabbix 网络不冲突。
- [ ] 四个常驻容器均为 `healthy`。
- [ ] `w8t-init-mysql` 为 `Exited (0)`。
- [ ] 浏览器能够访问 TCP 8088。
- [ ] 后端能够读取 Zabbix API 版本。
- [ ] Zabbix 数据源连接测试通过。
- [ ] 新告警、恢复和历史生命周期正常。
- [ ] 飞书告警和恢复卡片正文完整。
- [ ] MySQL 备份文件已生成并验证非空。
- [ ] Zabbix Token、飞书 Webhook 和数据库 Secret 未泄露。

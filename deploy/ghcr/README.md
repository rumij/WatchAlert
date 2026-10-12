# WatchAlert：统一从 GHCR 新部署

适用于全新 Linux amd64 服务器，Docker Compose v2。已有部署继续使用原 Compose 项目、配置与数据卷，不要用此文件替换原部署。

## 镜像清单

| GHCR 镜像 | 用途 | 版本 |
| --- | --- | --- |
| `ghcr.io/rumij/watchalert-backend` | 后端 | `release.env` 中固定的提交标签 |
| `ghcr.io/rumij/watchalert-frontend` | 前端，已含 Nginx | 同一提交标签；旧宿主机可加 `-compat` |
| `ghcr.io/rumij/watchalert-mysql` | MySQL 和一次性初始化容器复用 | `8.0.46` |
| `ghcr.io/rumij/watchalert-redis` | Redis | `7.4.11-alpine` |

不需要单独下载 Nginx、Go、Node、Python 或构建工具镜像。speedtest 不属于 WatchAlert 的依赖。新发布的 MySQL/Redis Package 首次需由所有者设为 Public 后，其他机器才能免登录拉取。

## 三步启动

下载本仓库 ZIP 后进入 `deploy/ghcr`；如果使用独立部署 ZIP，进入解压后的 `watchalert-ghcr` 目录。脚本需要 Bash 和 openssl。

```bash
bash prepare.sh
docker compose pull
docker compose up -d --wait
```

每一步成功后再执行下一步。`prepare.sh` 在本机生成随机数据库密码和后端配置，不打印密码。已有 `.env` 或 `config/config.yaml` 时会拒绝覆盖。初始化文件后可在 `.env` 修改 `WATCHALERT_HTTP_PORT`（默认 8088）和监听地址，再启动服务。首次访问 `http://服务器IP:8088`，按页面设置管理员密码；该端口仅向可信管理网络开放。

MySQL/Redis 不映射宿主机端口，使用 Compose 命名数据卷；数据库初始化复用 MySQL 镜像，并在后端建表成功后导入模板和默认租户。`init-mysql` 显示 `Exited (0)` 为正常状态。此包不包含任何实际告警、Zabbix 地址、API Token 或生产密码。

```bash
docker compose ps -a
docker compose logs --tail=50 w8t-service w8t-web init-mysql
```

请同时保存整个部署目录（包含私有 `.env` 和 `config/`）并另行备份数据库。重启/更新时沿用同一目录和 Compose 项目名，避免连接到新建的空数据卷；不要使用 `down -v`。

## 旧宿主机前端兼容选项

若旧内核 Docker 环境存在 Nginx `pwrite() /run/nginx.pid: Operation not permitted`，初始化配置后，在 `.env` 把 `WATCHALERT_FRONTEND_SUFFIX=` 改成 `WATCHALERT_FRONTEND_SUFFIX=-compat`，再执行拉取和启动命令。兼容镜像已在 GHCR 构建，不需要在服务器现场编译。

兼容版沿用固定的 Nginx 1.27.5 运行底座，标准版用于现代宿主机。旧底座不是长期安全更新方案，仍需安排宿主机维护。兼容性验证范围以发布记录为准，不宣称适用于所有旧内核。

## 依赖来源与维护

数据库镜像仅添加本仓库的来源标签，文件系统层、入口和上游许可证均保持继承，不从运行中的生产容器制作镜像。固定来源：

- MySQL 8.0.46：`docker.io/library/mysql@sha256:7dcddc01f13bab2f15cde676d44d01f61fc9f99fe7785e86196dfc07d358ae2b`，上游 <https://github.com/docker-library/mysql>。
- Redis 7.4.11：`docker.io/library/redis@sha256:ff02b58f971e7d7d156a1267e283fcbbeee91773b6aa36c49dac28ecfe28eadf`，上游 <https://github.com/redis/docker-library-redis>。
- 兼容前端运行底座：`docker.io/library/nginx@sha256:62223d644fa234c3a1cc785ee14242ec47a77364226f1c811d2f669f96dc2ac8`，上游 <https://github.com/nginx/docker-nginx>。

依赖更新需明确修改 `.github/workflows/ghcr-dependencies.yml` 中的版本与 digest，并验证新部署后再发布；不会因上游浮动标签变化而自动升级数据库。`8.0` / `7.4-alpine` 仅为便捷别名，本部署文件使用完整小版本。

维护者可运行 `python3 deploy/ghcr/package.py --tag sha-完整提交号 --output dist/watchalert-ghcr.zip` 生成只含白名单部署文件和初始 SQL 的独立包，不打包本机密码或数据。

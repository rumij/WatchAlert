# GHCR 镜像更新

本仓库发布配套的 Zabbix 特别版镜像（目前为 `linux/amd64`）：

- `ghcr.io/rumij/watchalert-backend`
- `ghcr.io/rumij/watchalert-frontend`

每次 `main` 的代码更新由 [GitHub Actions](https://github.com/rumij/WatchAlert/actions/workflows/ghcr.yml) 从源码构建。两个镜像均构建成功后才更新 `latest`；两个标签更新不具备跨镜像原子性，正式升级请固定同一个 `sha-<完整提交号>` 标签。首次发布还需要把两个 GHCR Package 设为 Public，之后可免登录拉取。

## 已有安装：拉取、重建两个应用容器

在原部署目录操作，下载本仓库的 `deploy/docker-compose/upgrade-ghcr.yaml` 到该目录。保留原有 `.env`、配置文件、挂载和 Compose 项目名。下例假设原服务名为 `w8t-service` / `w8t-web`，原文件为 `docker-compose.yaml`；如果使用了 `-p`、`--env-file` 或其他文件名，请沿用原参数。

先在 Actions 的成功运行摘要中复制配套的 `sha-...` 标签，设置后执行两条命令：

```bash
export WATCHALERT_RELEASE_TAG=sha-这里替换为完整提交号
docker compose -f docker-compose.yaml -f upgrade-ghcr.yaml pull w8t-service w8t-web
docker compose -f docker-compose.yaml -f upgrade-ghcr.yaml up -d --no-deps --no-build w8t-service w8t-web
```

只在两张镜像都拉取成功后执行第二条。未设置版本时使用 `latest`，便于测试；不要重启或重建 MySQL / Redis，也不需要 `down`。升级前备份现有数据库和配置，记录两张旧镜像及原 Compose 参数，升级后检查服务日志、登录和静默功能。需要回滚时使用原来的镜像配置重新创建这两个服务，数据库变更的恢复另按备份方案处理。

若原服务名为 `backend` / `web`，将覆盖文件中的两个服务键及 `WATCHALERT_BACKEND_HOST` 改为对应名称，并使用相同服务名执行上述命令。

非常旧的 Docker / seccomp 环境如出现 `pwrite() /run/nginx.pid: Operation not permitted`，仍按 [兼容前端重建说明](container-upgrade-20261010.md) 处理；公网镜像不等于已验证所有旧版宿主机兼容性。

本次静默修复及镜像在隔离环境验证，实际生产升级仍需在你的部署环境执行以上命令并验收。

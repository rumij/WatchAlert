# Zabbix 静默版容器升级方案（2026-10-10）

## 版本与适用范围

源码仓库：<https://github.com/rumij/WatchAlert>。成对镜像标签：`zabbix-silence-20261010`。本次包括单条、跨页勾选多条、设备、多个设备、设备组范围静默；支持自选时间、编辑、取消及生效边界校验；修复多个问题共用一次恢复事件时漏恢复、漏历史的问题。静默抑制通知，不会从 Zabbix 删除问题事件。

本次提交源码和构建入口，**未向 Docker Hub 发布镜像**。不能使用 `cairry/watchalert:latest` 代替本版本。Zabbix 的“单个/多重”是源端触发器配置，需按模板保持已有设置；本升级不会清理历史遗留问题，也没有新增首页告警聚合功能。

适用已有 Docker Compose 部署，先确认真实服务名、项目名、配置和数据挂载。本仓库覆盖文件对应 `w8t-service` / `w8t-web`；本地验证覆盖文件对应 `backend` / `web`。不同名称需按实际修改覆盖文件。**不把新安装 compose 覆盖到现有生产部署，不执行 down -v，不重跑初始化数据库服务。**

## 1. 构建或离线打包

在联网构建机检出本次提交，从仓库根目录执行（需要 Docker 和 Bash）：

```bash
bash deploy/docker-compose/build-app-images.sh zabbix-silence-20261010
```

生成 `dist/zabbix-silence-20261010/`，含前后端镜像 tar、SHA256SUMS、两种服务名的覆盖文件及 RELEASE_TAG。依赖源默认 registry.npmjs.org；联网构建机需要镜像源时可设置 `WATCHALERT_NPM_REGISTRY=https://registry.npmmirror.com`。默认 linux/amd64；其他架构需显式设置 WATCHALERT_PLATFORM 并另行验证。复制整个目录到目标主机，校验并加载：

```bash
cd /path/to/zabbix-silence-20261010
sha256sum -c SHA256SUMS
docker image load -i watchalert-app-images.tar
```

若镜像构建遇到网络问题，解决构建机访问镜像仓库或依赖源的问题后重建；不替换成未经核验的上游镜像。离线包不含实际配置、数据库和 Token。

## 2. 确认现有部署并准备回退

在**原部署目录**使用原来所有 compose 文件、项目名、env 文件。以下示例假定已有 `compose.yaml`，项目名由现有 `.env` 或目录确定；原部署使用 `-p` / `--env-file` 时，下文每条命令都沿用它们。

```bash
docker compose -f compose.yaml ps
```

核对后端的数据库类型、数据库路径、配置文件挂载，记录后端和前端当前镜像 ID。不要输出配置正文、Token 或密码。用旧镜像 ID 建立回退标签，并保存到私有备份目录：

```bash
mkdir -p /private/backup/watchalert-before-silence
backend_id=$(docker compose -f compose.yaml ps -q w8t-service)
web_id=$(docker compose -f compose.yaml ps -q w8t-web)
test -n "$backend_id" && test -n "$web_id"
docker image tag "$(docker inspect --format '{{.Image}}' "$backend_id")" watchalert-backend:before-silence-20261010
docker image tag "$(docker inspect --format '{{.Image}}' "$web_id")" watchalert-frontend:before-silence-20261010
docker image save -o /private/backup/watchalert-before-silence/old-app-images.tar watchalert-backend:before-silence-20261010 watchalert-frontend:before-silence-20261010
```

备份原 compose 文件、私有配置、`.env` 和 secrets，限制目录权限。记录 MySQL/Redis 的挂载位置；确认没有把数据放在容器可写层中。SQLite 部署必须保留 `/app/data` 持久化挂载；如果没有，先停后端，用 `docker cp` 备份实际数据库目录，并为新容器补上挂载后再继续。

## 3. 维护窗口与数据备份

暂停后端和前端，数据库及 Redis 继续运行；安排短维护窗口。轮询短暂停顿会在启动后按水位补偿，但仍需检查停机期间的源端事件。

```bash
docker compose -f compose.yaml stop w8t-service w8t-web
```

- **MySQL**：使用现有数据库管理员凭据完成一致性逻辑备份，包含例程、触发器和事件；备份成功后再升级。下面仅适用于现有 MySQL 容器有 `/run/secrets/mysql_root_password` 或 `MYSQL_ROOT_PASSWORD` 的部署。若凭据方式不同，使用现有备份工具，不另建账号或改密码。
- **SQLite**：后端停止后复制整个数据库目录（包括存在的 WAL/SHM 文件），再检查备份文件可读；不要只复制运行中的 `.db`。
- **Redis**：按已有认证方式执行持久化快照，检查成功后备份数据挂载目录。Redis 未认证才可直接使用下列 `redis-cli`；有认证则使用现有凭据注入方式。若还有外部写入者，需同时安排一致性备份。

MySQL 示例（密码不写入命令参数，备份文件为敏感数据）：

```bash
docker compose -f compose.yaml exec -T w8t-mysql sh -c '
  if [ -r /run/secrets/mysql_root_password ]; then
    MYSQL_PWD=$(cat /run/secrets/mysql_root_password)
  else
    MYSQL_PWD=${MYSQL_ROOT_PASSWORD:?Missing existing database credential}
  fi
  export MYSQL_PWD
  exec mysqldump -uroot --single-transaction --routines --triggers --events --all-databases
' > /private/backup/watchalert-before-silence/mysql.sql
test -s /private/backup/watchalert-before-silence/mysql.sql
```

未认证 Redis 示例：

```bash
docker compose -f compose.yaml exec -T w8t-redis redis-cli SAVE
```

复制实际 Redis 持久化目录到私有备份目录，并确认备份完成。**任何备份命令失败都先处理失败，不继续升级。**

## 4. 只替换前后端

将升级包中的 `upgrade-app.yaml` 放在原部署目录；保持原 compose 和所有实际配置。可先本地检查合并后的配置，注意其可能包含密码，不要上传输出。

```bash
WATCHALERT_RELEASE_TAG=zabbix-silence-20261010 docker compose -f compose.yaml -f upgrade-app.yaml config --quiet
WATCHALERT_RELEASE_TAG=zabbix-silence-20261010 docker compose -f compose.yaml -f upgrade-app.yaml up -d --no-deps --no-build --pull never --force-recreate w8t-service w8t-web
```

覆盖文件只更换应用镜像和前端代理目标，沿用原端口、挂载、网络及数据库。此变更没有删除列或清空数据的迁移；新容器仍应先在备份后启动。现有前端服务如果挂载旧的 nginx 配置或 `/usr/share/nginx/html`，该挂载会覆盖新镜像文件：升级前核对并将 nginx 代理保留到实际后端，新版本静态页面使用镜像自带内容。

当前本地验证项目使用 `backend/web` 服务名和 localhost:18088。升级该项目时改用 `upgrade-validation.yaml`，沿用其原 compose，命令末尾服务名改为 `backend web`；不要使用生产覆盖文件启动另一套数据库。

## 5. 验收

```bash
docker compose -f compose.yaml -f upgrade-app.yaml ps
```

按实际地址检查后端 `/hello` 返回正常，前端登录、数据源和当前告警可读。浏览器强制刷新以加载新前端。

1. 单条静默；勾选跨页多条后批量静默；一个设备、多个设备和一个设备组分别检查条件，确认未匹配的设备与其他数据源不被静默。
2. 自选开始/结束时间、未来计划、到期恢复通知资格；编辑及取消后生效；空条件和无效正则被拒绝。使用测试告警/测试通知渠道，避免实发通知。
3. 多个问题共用同一恢复事件时，全部离开当前告警并各有一条历史记录；重复采样不新增相同历史。新的问题事件仍会生成独立历史，不能仅按相同告警名称判断重复。
4. 保留关键等级告警，检查源端 P0/P1 对应告警仍显示。源端已经改为“单个”的触发器，至少观察两个采样周期，确认持续故障没有反复产生新的问题事件。
5. 恢复原来通知设置前核验接收渠道。此前容器测试关闭了通知发送，因此真实通知投递需要在升级维护窗口单独验收。

## 6. 回滚

前端和后端成对回退，沿用原配置、网络和数据。建立 `rollback-app.yaml`：

```yaml
services:
  w8t-service:
    image: watchalert-backend:before-silence-20261010
  w8t-web:
    image: watchalert-frontend:before-silence-20261010
```

```bash
docker compose -f compose.yaml -f rollback-app.yaml up -d --no-deps --no-build --pull never --force-recreate w8t-service w8t-web
```

本地验证项目同样替换为 `backend/web` 服务名。镜像回滚不主动恢复数据库；只有确认数据损坏或不兼容时才在停写后按已有恢复流程恢复数据库及缓存，明确升级后新数据可能被覆盖。不默认执行数据库还原。

## 验证证据与限制

功能此前已完成容器与浏览器验证：跨页多条、设备、多个设备、设备组四种范围；234 条已恢复告警全部补齐恢复历史（234 个唯一问题事件，未发现重复）；13 条关键告警保留。恢复共享事件的回归测试在修复前失败、修复后通过。验证过程中未向 Zabbix 写配置，实际触发器“单个”由用户在源端修改。

Go 相关包测试使用 `-vet=off`，仓库既有 notice/probing vet 问题未在本次修改；前端保留既有构建警告。生产环境升级和真实通知投递尚未执行。完整导入前端基线未做全仓业务审计；本次审查聚焦静默功能、恢复同步和发布入口，见发布审查记录。

参考：[Docker Compose up](https://docs.docker.com/reference/cli/docker/compose/up/)、[镜像离线保存](https://docs.docker.com/reference/cli/docker/image/save/)、[卷备份与恢复](https://docs.docker.com/engine/storage/volumes/#back-up-restore-or-migrate-data-volumes)。

## 旧服务器前端反复重启的兼容修复

若后端健康，但前端日志出现 `pwrite() "/run/nginx.pid" failed (1: Operation not permitted)`，不应将 Started 当作前端验收通过。较新的 Alpine/musl 可能使用被旧 Docker/seccomp 策略拒绝的 pwritev2；仅凭报错还不能证明主机实际拦截方式。相同现象和调用差异见 [NGINX 官方镜像问题记录](https://github.com/nginx/docker-nginx/issues/1059)。

已有可正常运行的旧前端镜像时，可保留其运行底座，将新版静态页面及代理模板移入新标签。这样不需要关闭 seccomp、不增加 privileged，也不需要从互联网拉取另一套依赖。旧底座的后续安全更新仍需纳入主机维护；这是现有环境的兼容路径，不代表已经验证所有旧内核。

先按前文把旧镜像标记为 `watchalert-frontend:before-silence`，确认原镜像包含 `/usr/share/nginx/html` 和 `/docker-entrypoint.d/20-envsubst-on-templates.sh`。在有本仓库源码的机器运行：

```bash
bash deploy/docker-compose/rebuild-frontend-compat.sh
```

默认读取已经导入的新版页面镜像 `watchalert-frontend:zabbix-silence-20261010`，以本地已验证的旧运行底座 `watchalert-frontend:before-silence` 构建 `watchalert-frontend:zabbix-silence-20261010-compat`。三个镜像名也可按顺序显式传给脚本；输出标签不能覆盖任一输入镜像。该脚本只构建镜像，不改正在运行的容器。

建立一个只覆盖前端的文件 `frontend-compat.yaml`：

```yaml
services:
  w8t-web:
    image: watchalert-frontend:zabbix-silence-20261010-compat
```

沿用本机已有的升级文件；若此前通过 `upgrade-silence.yaml` 适配了 config.yaml.template 的旧部署，必须保留它。以下示例使用该文件：

```bash
docker compose -f docker-compose.yml -f upgrade-silence.yaml -f frontend-compat.yaml config --quiet
docker compose -f docker-compose.yml -f upgrade-silence.yaml -f frontend-compat.yaml up -d --no-deps --no-build --pull never --force-recreate w8t-web
```

只有前端重新创建，后端与数据服务继续运行。等待后查看 ps、前端最新日志，再通过实际 Web 端口核验 /login HTTP 200、登录和静默页面。之后所有更新命令均沿用这三个配置文件，否则前端可能被切回原镜像。

本地以拒绝 pwritev2 并返回 EPERM 的测试策略重现了原前端退出码 1；兼容镜像在相同策略下保持运行。此策略仅用于本地故障模拟，不提供给生产部署使用。服务器上的实际旧内核与 Docker 仍需用户执行后验证。

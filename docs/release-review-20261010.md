# 发布审查记录（2026-10-10）

本次发布：Zabbix 自选时间静默、批量/设备/设备组快捷范围、共享恢复事件修复，以及前后端同仓容器升级入口。

采用 Alibaba Open Code Review delegate，实际运行 workspace preview 和对其全部 reviewable 路径的 rule 解析；使用系统 default、Go、JS/React、YAML、package.json 与 JSON 规则。由当前 Codex 模型结合 diff 与上下文审查，无独立模型 API 调用。所有 preview 的 `(path,status)` 逐项记录在 [发布文件清单](release-review-20261010.csv)，包含 OCR 默认排除项和原因。

完整前端作为配套源码首次纳入本仓库；此前测试目录中的 src/public 共 169 文件逐字节一致。仅对静默功能变更、构建入口、依赖声明和服务配置进行业务审查。未改的导入基线、图标等明确跳过完整业务审查；它们通过发布快照凭据扫描及整体生产构建，不据此宣称全仓业务安全审计通过。OCR 默认排除的本次回归测试另做人工补充检查。

## 问题及修复

| 严重级别 | 类别 | 文件与位置 | 证据及处理 |
|---|---|---|---|
| Medium | bug | `.dockerignore:27` | 打包脚本输出离线 tar 到 dist，后端构建原本会 COPY 整个构建上下文；再次构建可能将旧包带进镜像。已排除 dist 和 node_modules，避免体积膨胀和旧包进入新镜像。 |

| Medium | bug | `web/package-lock.json:657` | 导入锁定文件有 42 个空依赖条目，干净构建出现 damaged lockfile 警告。已用 npm 重新解析缺失条目及布局；保留路径上的既有版本均未变化，随后重新进行容器构建和测试。 |

已审查范围未发现尚未解决的 Critical/High 问题。跳过的导入基线不包含此结论。

## 验证

- Go：alert/mute、alert/consumer、alert/zabbix、internal/models、internal/repo、internal/services 六包通过；使用 `-work -vet=off -count=1`。测试为假服务/内存数据库；本轮未设置在线 Zabbix 测试凭据。
- 前端：静默两套测试、13 项全部通过；导入源码与被测目录一致。
- Compose：原生产 compose 与应用覆盖文件合并成功，数据库镜像、配置挂载、前端端口保留。
- Bash：构建脚本语法检查通过。
- 凭据：Gitleaks v8.30.0 对本次发布文件快照扫描，未发现泄漏；实际配置、数据库、Token 和运行日志不纳入提交。
- 先前容器/浏览器功能验证：四种批量范围匹配通过，234 条恢复记录唯一，13 条关键告警保留。实际通知发送未验证。

total_files=201, reviewed_files=39, skipped_files=162, coverage_rate=19.40%。前后端 linux/amd64 生产镜像均构建成功；修复锁文件后容器内 13 项前端测试通过，nginx 配置检查、登录页 HTTP 200 和 API 代理认证响应 401 通过。后端使用全新隔离 SQLite 与 Redis 启动，/hello HTTP 200；离线双镜像包已导出并生成 SHA256 校验文件。仓库既有 vet 与前端构建警告不在本次修复范围。生产升级未执行。

发布格式检查：后端及发布脚本无空白错误；首次导入的前端基线存在既有尾随空白，未做全仓格式化。

## 旧 Docker 前端兼容修复补充审查

新增范围：web/Dockerfile.compat、deploy/docker-compose/rebuild-frontend-compat.sh、升级文档及本记录。重新运行 OCR delegate preview/rule，按实际解析的 default 规则审查脚本；OCR 默认排除的两份 Markdown 和 Dockerfile.compat 另做人工补充审查，并核对文档内 YAML 与实际业务条件：本地镜像预检、单独输出标签、静态资源及代理模板完整复制、构建不修改运行容器、仅重新创建前端、保持原配置和备份。total_files=4, reviewed_files=4, skipped_files=0, coverage_rate=100%。没有未解决的 Critical/High 问题。

- 故障模拟：仅在本地测试容器中拒绝 pwritev2 并返回 EPERM；原 Alpine 前端出现同样 pwrite/nginx.pid 错误，退出码 1。
- 兼容镜像：保留既有 nginx/1.27.5 运行底座，原样复制新版前端静态资源及模板；相同故障策略下持续运行，nginx -t、/login HTML 和 API 401 认证响应通过。
- 脚本通过 bash -n 和实际本地镜像构建；旧镜像静态目录和模板处理脚本存在。未重新运行无修改的 Go/React 业务测试。
- 尚未在用户的 Linux 3.10.0-957 主机上实际验证；主机原因依据日志与官方问题记录判断，并非已取得系统调用跟踪。保留旧底座的安全维护责任，没有关闭生产 seccomp 或增加 privileged。

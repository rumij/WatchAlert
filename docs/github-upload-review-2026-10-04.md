# GitHub 上传准备检查（2026-10-04）

本次任务：将本地 WatchAlert 后端源码上传到 GitHub 私有仓库，排除运行凭据和数据。保留原项目许可证和来源说明。本次没有访问生产设备、发送通知、轮询实际数据源或发布 Docker 镜像。

## 脱敏与排除

- 实际 `config/config.yaml` 和 `data/w8t.db` 未修改，通过 SHA256 与任务开始时的文件逐一比对；它们以及日志、Secret、`.env`、数据库备份和可执行文件均不提交。
- 上传 `config/config.example.yaml` 与空值 `.env.example`；Compose 密码从私有 `.env` 读取，缺失时拒绝解析。
- Helm 默认密码为空，必须从忽略的私有 `values.local.yaml` 提供；后端与初始化 Job 沿用数据库密码。配置节使用当前后端支持的 `Database`。
- AI 测试中的硬编码 API Key 已删除；ClickHouse 测试中的固定密码和实际地址已删除。AI、ClickHouse、Elasticsearch 集成测试只有明确提供环境变量才运行。
- 文档中的实际 Zabbix 地址替换成文档保留地址，删除 README 中演示登录密码。
- 增加 Docker 构建上下文排除，镜像发布工作流改为手动触发并限制权限。GitHub 上传不自动执行镜像发布。

## OCR delegate 检查

使用 Alibaba Open Code Review CLI v1.12.11 的 `delegate preview --format json` 和 `delegate rule --format json <paths...>`；规则来自 OCR 内置规则解析结果，没有设置自定义覆盖规则，也没有配置外部模型 API。

此次目录原先没有 Git，按上传授权初始化 Git，并以不含文件的空提交建立 HEAD。因此 preview 将首次导入文件都列为 added，不能将它们全部视为本次业务代码修改。

| 字段 | 数值 |
| --- | ---: |
| preview_total_files（含 OCR 过滤项） | 326 |
| excluded_files（OCR 默认过滤） | 44 |
| total_files（preview reviewable 条目） | 282 |
| reviewed_files | 8 |
| skipped_files | 274 |
| coverage_rate | 2.84% |

全部 `(path, status)` 条目的检查或跳过原因见 [CSV 清单](github-upload-review-2026-10-04.csv)。跳过项属于既有代码导入，已参加凭据扫描，但没有逐文件完整业务逻辑审查；不把凭据扫描计为完整代码评审。

OCR 默认过滤 `*_test.go`。另人工检查本次修改的四个测试文件：`pkg/ai/ai_client_test.go`、`pkg/provider/test/clickhouse_test.go`、`pkg/provider/test/logs_elasticsearch_test.go`、`pkg/provider/test/kubernetes_test.go`。最后一个文件仅将触发 vet 失败的动态格式串日志改为普通日志调用，以完成相关测试包检查。README、脱敏部署手册和 `.env.example` 也做了相应人工检查。

## 问题与处理

| 严重级别 | 类别 | 文件与行号 | 证据 | 处理 |
| --- | --- | --- | --- | --- |
| 高 | security | `pkg/ai/ai_client_test.go:18`、`:45` | 原代码包含硬编码 API Key，外部服务测试默认会尝试请求 | 已改为环境变量，并在缺失配置时跳过；若原 Key 曾实际使用，应由其所有者作废或轮换 |
| 高 | security | `.gitignore:31`、`:45` | 原忽略规则没有排除运行配置、SQLite 数据库和编译产物 | 已添加排除，并验证实际运行文件不在提交清单 |
| 中 | security | `deploy/docker-compose/docker-compose.yaml:62`、`:78`；`deploy/helmchart/values.yaml:31`、`:173`、`:184`；`pkg/provider/test/clickhouse_test.go:18` | 固定部署密码和测试密码原本会随源码上传 | 已删除上传内容中的固定密码，改为私有配置或环境变量 |
| 中 | security | `docs/WatchAlert-Zabbix-离线部署手册.md:14`；`pkg/provider/test/logs_elasticsearch_test.go:56` | 文档和测试存在实际内网地址 | 已替换为文档示例或环境变量 |

以上行号对应处理后的文件；证据只描述原问题，不复述秘密值。本次明确问题已修复。既有 [安全审查报告](open-code-review-2026-09-23.md) 中的鉴权等高风险项不在本次上传准备的修复范围内，也没有在本次得到完整复核，不能据此宣称项目已通过完整安全审计。

## 离线验证

- Gitleaks v8.30.1 对实际待上传文件扫描，没有发现匹配其规则的凭据泄露；此结果不保证能识别所有类型的敏感信息。
- 实际运行配置和数据库 SHA256 保持一致；运行配置、数据库、Compose `.env`、可执行文件的 Git 忽略检查通过。
- 配置模板、Compose、Helm values、GitHub workflow 的 YAML 解析通过。
- Docker Compose `config --quiet` 通过，未启动或重建容器。Compose 提示已有 `version` 字段过时，该提示不影响解析。
- Helm v4.3.0 用无效的检查占位密码离线渲染了 13 个资源；确认后端和初始化 Job 密码传递一致，缺失密码会拒绝渲染。未连接 Kubernetes 集群。
- Go 1.24.11 对 `pkg/ai`、`pkg/provider/test` 的四个相关测试进行了定向检查，两个测试包通过；四个外部集成测试按预期 SKIP，验证了默认不访问外部服务。首次 vet 失败已修复后重新检查通过；期间一次 Windows 临时文件占用错误也在重查时消失。
- 未运行全部业务测试、外部服务联调或生产部署验证；仓库不含前端源码和离线镜像包。

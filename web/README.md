# WatchAlert 配套前端

基于 w8t-io/WatchAlert-web 的本地源码，随本仓库 Zabbix 特别版一起维护；不是上游官方发布。本仓库沿用根目录 AGPL-3.0 许可。

包含单条静默、跨页勾选后批量静默、按设备/多个设备/设备组匹配、自选开始与结束时间，以及静默查询、编辑和取消。匹配条件带数据源边界；设备范围仍需检查预览条件。

开发：`npm ci --legacy-peer-deps` 后 `npm start`，开发代理使用 localhost:9001。验证：`CI=true npm test -- --watchAll=false --runInBand`。生产构建：`docker build -t watchalert-frontend:zabbix-silence-20261010 web`（从仓库根目录执行）。容器默认代理到 w8t-service:9001，可通过 WATCHALERT_BACKEND_HOST 和 WATCHALERT_BACKEND_PORT 修改。

生产部署与回滚见 [容器升级方案](../docs/container-upgrade-20261010.md)。生产构建关闭源码映射；现有前端仍存在依赖/构建警告，不代表完成全项目依赖升级。

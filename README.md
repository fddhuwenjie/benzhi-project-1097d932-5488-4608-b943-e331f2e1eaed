# 无障碍出版放行台

无障碍出版放行台面向数字出版质量团队，用一条可追溯业务链管理待发布版本的无障碍合规修复与放行。负责人先建立出版个案并冻结规则基线，审查员登记发现，修复编辑逐项举证，独立复核员在职责分离校验后批准或退回，最后签发发布凭证并冻结可验证的证据归档。

工作台支持冻结前基线影响预检；审查发现可按有界批次原子登记并拦截规范化重复定位；同一发现项的修复证据按轮次只追加，失败后必须沿上一轮摘要继续；复核退回使用关联具体发现项的整改清单，并由新一轮有效证据逐项闭环。发布后可实时核验凭证与当前个案、基线、内容和最终批准的绑定，归档后可逐组件复验并下载文件名稳定、内容确定的完整 JSON 证据清单。

状态依次为 `DRAFT`、`PROFILE_FROZEN`、`AUDITING`、`REMEDIATING`、`REVIEW_PENDING`、`APPROVED`、`RELEASED` 和 `ARCHIVED`。每个写请求均要求唯一 `request_id`；建档后还需携带 `expected_revision`，用来阻止过期写入。业务数据、幂等结果、发布凭证和摘要链事件保存在本地 SQLite。

## 构建

```bash
go build ./cmd/server
```

## 运行

```bash
go run ./cmd/server -addr=127.0.0.1:19137 -db=accessibility_release.db
```

打开 `http://127.0.0.1:19137/` 即可使用浏览器工作台。未提供 `-addr` 时，服务先读取 `PORT` 并绑定 `127.0.0.1:<PORT>`；若 `PORT` 也为空，则使用 `127.0.0.1:19137`。服务拒绝默认绑定未指定地址。

## 测试与自检

运行全部测试：

```bash
go test ./...
```

真实 HTTP 全流程自检会创建临时 SQLite 数据库，完成建档、冻结、审查、整改、独立复核、发布与归档，然后主动关闭：

```bash
go run ./cmd/server -self-check -addr=127.0.0.1:19137
```

## 代码结构

- `internal/accessibility`：领域实体、状态机、不变量、稳定摘要和归档证据规范化。
- `internal/audit`：只追加审计事件、载荷规范化、摘要链生成与校验。
- `internal/workflow`：业务用例、职责分离、幂等、乐观并发和发布策略。
- `internal/store`：SQLite 模式、事务仓储、规范化证据图和启动完整性检查。
- `internal/web`：原生 HTML/CSS/JavaScript 工作台、同源 JSON API 和安全中间件。
- `cmd/server`：配置、依赖装配、优雅关闭和真实 HTTP 自检。

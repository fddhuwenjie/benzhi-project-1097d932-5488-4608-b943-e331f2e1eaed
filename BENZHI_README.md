# BENZHI_README

## 项目说明
- 项目：benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed
- 项目用途：已完整实现无障碍数字出版物从版本建档、规则基线冻结、问题审查、修复举证、职责分离复核、发布授权到确定性证据归档的单一业务闭环，并提供 SQLite 事务持久化、摘要链校验、浏览器工作台和真实 HTTP 全流程自检。
- Go 工具链：`golang:1.23.0`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：无障碍出版放行台
- 项目介绍：面向数字出版质量团队的无障碍合规修复与发布放行应用，将一本待发布数字出版物从版本建档、规则基线冻结、问题审查、修复举证、独立复核推进到授权发布和证据归档，形成状态明确、可追溯且可验证的单一业务闭环。
- 项目概述：面向数字出版质量团队的无障碍合规修复与发布放行应用，将一本待发布数字出版物从版本建档、规则基线冻结、问题审查、修复举证、独立复核推进到授权发布和证据归档，形成状态明确、可追溯且可验证的单一业务闭环。
- 核心工作流：出版负责人创建待发布版本并冻结适用的无障碍要求基线，审查员登记并分级全部发现项，修复编辑逐项提交修复证据，独立复核员在职责分离校验后作出复核结论；全部阻断项关闭后系统签发发布授权，最后生成可校验的证据清单并把个案从 DRAFT 依次推进至 PROFILE_FROZEN、AUDITING、REMEDIATING、REVIEW_PENDING、APPROVED、RELEASED 和 ARCHIVED。
- 对外接口：由 Go 服务提供一个原生 HTML、CSS 和 JavaScript 浏览器工作台，包含个案总览、要求基线、发现项、修复证据、独立复核、发布授权和归档校验页面，并以同源 JSON API 驱动所有状态操作；不引入 Node 构建链。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...

cd /app && GOTOOLCHAIN=local go run ./cmd/server -self-check -addr=127.0.0.1:19137

cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh

./build_benzhi_docker.sh benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed-amd64 linux/amd64

./build_benzhi_docker.sh benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed-arm64 linux/arm64

docker run -it benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -self-check -addr=127.0.0.1:19137`

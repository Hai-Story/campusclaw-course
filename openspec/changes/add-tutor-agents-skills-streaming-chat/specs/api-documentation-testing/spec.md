## Purpose

让开发者和验收者通过与处理器注解同源的接口描述了解真实请求、响应及授权规则，在 Swagger 中实际调用，并以逐接口脚本、数据库核对及 HTML 报告留下可复现的第五课验证证据。

## ADDED Requirements

### Requirement: Annotation-generated OpenAPI and same-origin Swagger

系统 MUST 从处理器注解生成 OpenAPI 3 描述，通过无需登录的 `GET /openapi.json` 提供，并通过 `/swagger/` 或 `/swagger/index.html` 提供可使用的 Swagger UI。文档 MUST 包含现有主要接口及本课全部接口的路径、参数、请求/响应结构、成功码、实际错误码、教师要求和 Bearer security scheme。SSE 接口 MUST 声明 `text/event-stream` 与四类事件格式。受保护操作 MUST 使用 `Authorization: Bearer <token>`，不能要求 Cookie。生成 MUST 可重复执行，并能检测已提交描述与处理器注解不一致。

#### Scenario: Documented requests can actually be executed

- **WHEN** 使用者打开 Swagger，调用登录并在 Authorize 输入返回 Token，再调用智能体、技能、会话或检索接口
- **THEN** 请求发送到当前环境，参数和响应符合文档；未授权请求得到文档所列的 401

#### Scenario: Generator detects documentation drift

- **WHEN** 修改处理器注解后重新生成并进行一致性检查
- **THEN** 文档随注解更新；陈旧生成物、重复 operationId 或路径与方法缺失使检查失败

### Requirement: URL-organized API tests with database assertions

仓库 MUST 提供 `apitest/cases/`，按接口 URL 组织，每个方法一个可运行脚本，例如 `cases/api/login/POST.py`、`cases/api/conversations/{id}/messages/POST.py`。共享设施 MUST 不替代独立接口脚本。`apitest/run_all.py` MUST 一键执行并生成 HTML 报告；失败退出非零。测试 MUST 覆盖登录、知识检索、智能体提示词/绑定、技能增删改、创建会话及发送消息，同时核对响应内容与数据库实际状态，包括写入、权限拒绝无写入和消息/引用持久化。

#### Scenario: Full acceptance has traceable evidence

- **WHEN** 对隔离的测试应用和测试数据库运行全部接口脚本
- **THEN** 报告逐项记录通过或失败，验证成功响应内容及对应数据库值，并核对同班他人、跨班与未知对象的同形 404

#### Scenario: Response-only checks cannot count as complete

- **WHEN** 没有提供测试数据库连接、应用不可达或必需用例未执行
- **THEN** runner 非零退出并在报告说明未执行原因，不报告完整通过

### Requirement: Reproducible and private test artifacts

自动测试 MUST 使用独立本地测试数据及环境变量提供的测试配置，不读取现有私人 `.env` 或生产凭据。模型 mock MUST 验证真实的上游流式协议、技能策略差异、无证据生成和故障路径；mock 结果 MUST 明确标为链路验证，不替代真实模型行为验收。HTML 报告、临时数据库、缓存和真实 Token MUST 不入 Git；报告 MUST 对 Token、口令、连接凭据脱敏。

#### Scenario: Deterministic tests and real model validation are distinguishable

- **WHEN** 本地模拟网关验收通过，但尚未完成获授权真实网关的解题引导检查或同伴交叉验证
- **THEN** 验证记录分别列出已通过和待完成项，不把 mock 通过等同于模型行为与同伴验收通过

#### Scenario: Reports are safe to inspect

- **WHEN** 登录和消息用例写入 HTML 报告
- **THEN** 报告能查看断言结果与脱敏响应，不能查看实际 Token、口令或数据库凭据，且报告路径被 Git 忽略

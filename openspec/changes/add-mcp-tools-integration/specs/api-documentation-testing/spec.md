## MODIFIED Requirements

### Requirement: Annotation-generated OpenAPI and same-origin Swagger

系统 MUST 从处理器注解生成 OpenAPI 3，通过公开 `GET /openapi.json` 和 `/swagger/` 或 `/swagger/index.html` 提供同源文档。文档 MUST 包含现有主要接口、第五课接口及本次 MCP 管理/审计接口的路径、参数、请求/响应结构、成功码、实际错误码、教师要求和 Bearer security scheme。MCP 令牌输入 MUST 标为只写，响应 MUST 仅含 has_token。SSE MUST 声明 text/event-stream、meta/delta/done/error、重复 meta 的逐轮 tool_calls 及省略规则。受保护操作 MUST 使用 Authorization Bearer，不能要求 Cookie。生成 MUST 可重复执行并检测陈旧描述与路由漂移；自建 /mcp 的 JSON-RPC 方法及独立验收方法 MUST 有可查文档。

#### Scenario: Documented requests can actually be executed

- **WHEN** 使用者在 Swagger 登录并 Authorize，登记本班自建 MCP、连接、列工具、保存技能、查看审计
- **THEN** 请求与响应符合文档且不回传 MCP 令牌，未授权返回 401，学生管理操作返回 403；原智能体、会话、检索试调仍可使用

#### Scenario: Generator detects documentation drift

- **WHEN** 新增 MCP 路由或修改 SSE/技能注解后重新生成并检查
- **THEN** 文档同步更新；陈旧生成物、重复 operationId、缺路径方法或无效 schema 引用使检查失败

### Requirement: URL-organized API tests with database assertions

仓库 MUST 提供 apitest/cases，按接口 URL 组织、每个方法一个可运行脚本；共享设施不能替代独立脚本。apitest/run_all.py MUST 一键执行并生成 HTML 报告，失败非零。测试 MUST 保留登录、知识检索、智能体、技能、会话的原有覆盖，并新增 MCP 登记/详情/更新/删除、连接/工具/断开/审计的成功、401、403、跨班同形 404及无副作用断言；MCP 技能验证失败须核对数据库未部分更新。端到端 MUST 经真实自建服务发现、技能绑定、模型提出调用和对话回传，并核对工具缓存、授权、会话元信息与审计实际落库。

#### Scenario: Full acceptance has traceable evidence

- **WHEN** 对隔离应用、数据库和真实自建 MCP 运行全部接口脚本
- **THEN** 报告逐项呈现响应与数据库断言，至少有一条绑定 formula_hint@campus-tools 的对话产生 ok 审计，并覆盖断开后 denied、超时/业务错误后的 error、多轮和预算收敛

#### Scenario: Response-only checks cannot count as complete

- **WHEN** 数据库缺失、应用或自建服务不可达、必需用例未执行，或只用宿主 mock 代替真实 MCP 往返
- **THEN** runner 非零退出并记录原因，不报告完整通过

### Requirement: Reproducible and private test artifacts

自动测试 MUST 使用独立本地测试环境与环境变量提供的测试配置，不读取现有私人 .env 或生产凭据。模型 mock MUST 保留第五课真实 SSE、策略、无证据及故障测试，并增加结构化调用、参数分块、两轮及多调用、预算、恶意工具返回和未知工具响应。mock 仅验证链路，MUST 不替代获准真实模型的工具选择与提示语义验收。HTML 报告、临时数据库、缓存与真实 Token MUST 不入 Git；报告 MUST 转义不可信文本并隐藏登录 Token、MCP 令牌、口令和连接凭据。课程同伴交叉验证 MUST 分别核对服务器/工具隔离、令牌不回传、断开失效及审计范围，并留脱敏证据。

#### Scenario: Deterministic tests and real model validation are distinguishable

- **WHEN** mock 链路通过，但真实模型自主选择/多轮、提示不直接给答案或同伴核验尚未执行
- **THEN** 验证记录分别列出已通过和待完成，不将自测或多个自动账号当成真实模型或同伴签认

#### Scenario: Reports are safe to inspect

- **WHEN** 登录、服务器登记和工具调用结果写入 HTML 报告
- **THEN** 报告可核对断言与脱敏内容，不能看到实际令牌、口令或连接凭据；报告目录保持 Git 忽略

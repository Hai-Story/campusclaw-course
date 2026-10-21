## Why

CampusClaw 第五课已提供技能、班级隔离和私有流式会话，但技能工具仍只接受 `knowledge_search`，模型不能发现或使用外部学习工具。第六课需要在同一仓库交付真实 MCP 服务及宿主编排，使学生在教师限定的工具范围内获得可追溯的学习提示。

## What Changes

- 新增同仓 `mcp-server/`，通过现有 `docker-compose.yml` 构建启动，提供带 Bearer 鉴权的 HTTP JSON-RPC `/mcp`、公开 `/health`，以及至少两个只给学习提示的工具。
- 新增教师本班 MCP 服务器登记、维护、连接发现、断开和删除；发现清单持久化、整体替换；令牌仅在服务端保存和使用，响应只返回 `has_token`。
- 扩展技能工具为内置 `knowledge_search` 与 `tool@server` 引用，保存时核对本班服务器、连接和发现结果，运行时再次授权。
- 根据命中技能生成候选工具声明，由模型选择名称及参数，宿主按发现的 JSON Schema 校验并执行；支持多轮、轮次及累计次数/时间上限，失败回传并降级作答。
- 扩展现有 SSE `meta`，按轮发送 `tool_calls`，调用记录先于最终正文；新增本班教师审计和管理界面，保留既有检索、引用、会话隐私与取消语义。
- 补齐注解生成的 OpenAPI、Swagger、逐接口与数据库验收、独立真实 MCP 链路及同伴交叉验证记录。

## Capabilities

### New Capabilities

- `mcp-tool-server`：同仓、可独立验证的提示型 MCP HTTP 服务与协议边界。
- `mcp-server-management`：教师按班管理服务器、发现缓存、令牌边界与审计查询。
- `mcp-tool-orchestration`：候选工具声明、模型调用循环、参数校验、授权、预算与结果信任边界。

### Modified Capabilities

- `tutor-skills`：将第五课单一工具白名单扩展为可验证的 MCP 引用，保留其他文件和路由规则。
- `tutor-conversations`：增加工具调用元信息与多轮进度，保持私有历史、真实增量和失败收尾。
- `api-documentation-testing`：扩展 MCP 管理及工具对话的文档、接口/数据库与验收证据。

以上三个路径目前只存在于在途变更 `add-tutor-agents-skills-streaming-chat`，`openspec list --specs` 尚无主规约。本变更以其 delta 和当前代码为前置基线，使用相同路径记录 MODIFIED 条目；不复制成同义新能力，不改写第五课文件，不自动补勾或归档。未来同步/归档前需先使前置规约进入主规约，再应用本变更，避免 MODIFIED 条目找不到基线。

## Impact

- 预计影响 `backend/internal/tutor/`、模型网关适配、数据库迁移、配置与路由；新增 MCP 客户端模块和服务；前端沿用会话及教师设置入口。
- 预计新增 `/api/mcp/servers` 及详情/连接/断开/工具子路由、`/api/mcp/audits`；技能接口和会话 SSE 增量扩展，旧 `/api/ask` 不变。
- 新增服务器、发现工具与调用审计表；可选新增经固定版本评估的 JSON Schema 校验依赖，依赖仅安装在项目内。
- Compose 仅增加内部服务与环境变量占位说明，不发布公网工具端口、不读取现有私人 `.env`、不重置数据卷。
- 本次只形成规划文档，全部实施任务保持未完成；不部署、不修改实现代码。

## References

- [第六课课件及各知识点子页](https://devops.hello1023.com/课件/第6课-课件-MCP与工具接入/index.html)
- [第六课任务清单](https://devops.hello1023.com/课程任务清单-第6课.html)

按 2026-10-21 读取的版本规划。任务清单中“拼进用户消息”按课件补充要求解释为进入数据侧；实际循环用关联 `tool_call_id` 的 `tool` 消息，不提升为 system 指令。课件强调多轮调用，不能用固定预调用或只 mock 宿主替代。

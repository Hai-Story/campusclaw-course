## Purpose

让教师管理本班 MCP 服务连接和已发现工具，在不回传令牌的前提下明确展示可用状态与工具完整声明，并通过按班隔离的调用记录核对学生对话中的工具动作。

## ADDED Requirements

### Requirement: Teacher-only class-scoped server lifecycle

系统 MUST 提供教师专用 `GET/POST /api/mcp/servers`、`GET/PUT/DELETE /api/mcp/servers/{id}`。登记 MUST 接收名称、Base URL 和可选令牌，创建返回 201、更新 200、删除 204；名称 MUST 符合 `^[a-z][a-z0-9_-]{0,31}$` 且班内唯一。更新省略令牌表示保留，显式空串表示清除。Base URL MUST 为允许目标中的 HTTP(S) 地址，不接受 userinfo、query、fragment 或任意重定向；同仓内部服务地址 MUST 可登记。所有接口 MUST 要求应用 Bearer，学生包括只读操作返回 403；班级只取当前服务端会话，跨班编号和未知编号返回同形 404。

#### Scenario: Server secrets do not round-trip

- **WHEN** 教师登记、列出、读取或更新已配令牌的本班服务器
- **THEN** 响应仅含 `has_token` 等非秘密配置，从不含令牌值或可还原值；同名登记返回 400；省略令牌的更新不会清除已配令牌

#### Scenario: Management cannot be used to enumerate another class

- **WHEN** 学生请求管理接口，或教师比较他班服务器与未知服务器的详情、修改、删除、连接、断开及工具列表
- **THEN** 学生获得 403，教师两类编号获得完全相同的 404；均不发出远端请求或修改任何班级数据

### Requirement: Atomic connection discovery and immediate disconnect

系统 MUST 提供 `POST /api/mcp/servers/{id}/connect`、`POST /api/mcp/servers/{id}/disconnect` 与 `GET /api/mcp/servers/{id}/tools`。连接 MUST 以服务端令牌完成版本/能力握手、initialized 通知和发现，成功后原子整体替换缓存并标记 connected；重复连接不得重复追加。握手、发现、声明校验或持久化任一步失败 MUST 返回可读的 503 并使服务器处于 disconnected，不发布部分发现结果。失败保留的上次成功快照不得作为当前可用工具。断开 MUST 清空可用缓存并返回 200；更新名称、地址或令牌 MUST 同样使连接失效；删除后工具不可调用。对断开或删除完成后尚未发出的调用 MUST 拒绝，不将已完成的远端动作宣称已撤销。

#### Scenario: Refresh replaces rather than appends

- **WHEN** 连续连接两次，第二次远端增加或删除工具
- **THEN** 工具列表恰等于第二次成功发现的清单；连接中出现畸形声明则整体失败，不泄露部分清单为可用状态

#### Scenario: Failed reconnect and disconnect revoke execution

- **WHEN** 已连接服务器重连失败或被断开、改名、删除，随后旧技能再次触发
- **THEN** 无法使用旧缓存成功执行；对话明确报告 denied，连接失败也不阻断无 MCP 工具的既有对话

### Requirement: Visible descriptors and renewed tool approval

教师管理界面 MUST 展示本班服务器、连接状态、可读错误、完整工具说明与 inputSchema，令牌仅允许密码框输入并于成功提交后清空，不回显或存到浏览器持久存储。发现后工具说明或 inputSchema 改变 MUST 留下可查看的变更记录，通过教师专用 `GET /api/mcp/servers/{id}/changes` 有限分页读取；既有技能对该工具的授权 MUST 失效，直到教师查看新声明并明确重新保存相关技能。初次保存有效引用视为对该版声明授权。未知写入或外发副作用的工具 MUST 默认不可自动执行；本课只启用部署白名单内且明确批准的只读提示工具。

#### Scenario: Changed description cannot silently inherit approval

- **WHEN** 已绑定工具的说明或结构在重新发现后改变
- **THEN** 教师可查看变化，旧技能仍保留原文但该工具运行时为 denied；教师确认新声明并重新保存技能后方可恢复

### Requirement: Private class-level call audit

系统 MUST 提供教师专用 `GET /api/mcp/audits`，按时间及 ID 倒序、有限分页返回本班工具调用。每次能归属本班服务器的调用或尝试 MUST 留一条记录，含工具引用、服务器标识及名称快照、用户、会话、提问、调用轮次、时间、耗时和 `ok/error/denied`；成功含脱敏限长摘要，失败含固定或脱敏原因。服务器删除后旧记录 MUST 保留；已删除/改名引用无法归属或伪造他班引用时，只在本次应答显示 denied，不插入他班审计。审计 MUST 不暴露令牌、原始请求头、完整对话或完整工具参数；教师不得通过审计读取学生会话正文。

#### Scenario: All call outcomes can be traced

- **WHEN** 分别执行一次成功、一次远端超时或业务错误、一次本班已断开服务器调用
- **THEN** 产生 ok、error、denied 各一条审计，能定位用户、会话、轮次和服务器；学生不能读取，其他班教师列表不含这些记录

#### Scenario: Deletion preserves evidence without fabricating attribution

- **WHEN** 删除服务器后查看其历史审计，再使用旧引用提问
- **THEN** 历史记录仍在；本次 meta 含 denied，不伪造服务器身份或记录成 ok

## Context

动机和课程来源见 proposal.md。本次只规划，以下路径和数据结构均为拟实施设计。

已核对的代码基线：

- Go 1.22 API 与 React 前端，MySQL/Qdrant，通过 `docker-compose.yml` 单实例启动，目前无 MCP 服务。
- `backend/internal/tutor/skill.go` 只允许单个 knowledge_search，store 已将工具存为 tools_json，可继续保存引用数组；路由已确定性选择一个绑定且启用技能。
- tutor/stream.go 在检索后发一次 meta，knowledge/stream.go 仅解析正文 delta，HistoryTurn 只表示普通消息；尚不能表达 function 声明、assistant tool_calls 或 tool_call_id。
- tutor/prompt.go 保留最近六条用户/助手历史并限定绑定材料；无材料仍生成。该检索策略与旧 `/api/ask` 保持兼容。
- 前端 api.ts 已解析四类 SSE，TutorChat 当前每次 meta 会重置技能与引用，需新增工具集合并语义。
- 已有注解生成器、同源 Swagger、apitest 的 URL/方法脚本、隔离 MySQL fixture 和流式 mock；复用这些设施扩展，不另建一套测试系统。
- 第五课规约尚在 changes，未同步主规约，真实模型及同伴验收仍未勾选。新 delta 精确沿用原 Requirement 名称；本次不改第五课任务状态，后续同步有顺序依赖。

## Goals / Non-Goals

**Goals:**

- 用真实同仓服务贯通“登记→握手发现→技能批准→模型调用→回传生成→审计”。
- 将工具决策、协议执行和会话流分别放在网关适配、MCP 客户端、tutor 编排层，便于替换服务和独立测试。
- 无 MCP 的路径继续走原检索和真流式生成；有 MCP 时局部故障可解释、可追溯、可收敛。

**Non-Goals:**

- 不支持 stdio、旧双端点 HTTP+SSE、resources/prompts、远端进程管理、OAuth、通用写入工具和任意公网服务器接入。
- 不增加第八课的指标平台、分布式 tracing、输出侧外发意图扫描或多副本部署；本课实施已知载荷检查与调用日志。
- 不用本次规划替代第五课未完成验收；不部署、清数据卷或自动归档。

## Decisions

### 1. 独立 Go 服务，限定 HTTP 协议范围

新增 `mcp-server/` 独立 Go module、Dockerfile 和 .dockerignore；尽量复用后端现有 Go 构建方式，用标准库提供请求路由、JSON-RPC 和两个确定性提示工具。相比分拆第二仓，保持服务、宿主和测试能在同一提交内审查；相比引入完整多能力框架，有限协议子集更适合本课边界。

固定支持 MCP `2025-06-18` 版本的无状态 HTTP JSON 子集，initialize 声明 tools、完成 notifications/initialized，后续请求带 MCP-Protocol-Version。宿主发送 Accept: application/json, text/event-stream；自建服务返回 JSON，GET /mcp 返回 405，不分配会话 ID。宿主适配器对 POST 返回的 JSON 与 SSE 中的单个 JSON-RPC 结果均有受限解码，但不实现后台通知流、恢复与重连。

按 [官方传输](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports)、[生命周期](https://modelcontextprotocol.io/specification/2025-06-18/basic/lifecycle) 和 [工具](https://modelcontextprotocol.io/specification/2025-06-18/server/tools) 补全课件未展开的通知确认、版本头、ID、错误与 content/isError 行为；这是选定兼容版本，不宣称是最新版本或完整 MCP 实现。POST 设置消息/响应大小上限；存在 Origin 时只接受显式允许来源，无 Origin 的服务间请求正常处理。

工具示例：formula_hint 接收 query（1–500 字）给步骤检查方向；unit_convert 接收 query、from_unit、to_unit，给同维度单位关系与运算方向，不返回题目最终数值。inputSchema 声明 object、required、properties 和 additionalProperties:false；服务端和宿主均校验，不执行任意表达式或外部 I/O。

### 2. 内部网络与服务端秘密

Compose 新增 mcp-demo:8090，只连现有内部网络，不配置 ports，不成为 API 启动健康依赖；MCP 不可达不得拖垮应用启动。MCP_AUTH_TOKEN 仅运行时传入服务；教师通过密码框把同一值登记到宿主，服务端数据库的专用秘密字段持久化。响应 DTO 从结构上排除此字段，只生成 has_token；网关提示、审计、错误与报告也不用数据库行直接序列化。

该方案沿用课程的服务端保存口径，不引入本课以外的密钥管理服务；数据库备份仍需按秘密存储管理。API 不自动把应用登录 Token 或网关 key 当成 MCP 令牌。更新 token 字段缺省/空串的含义显式区分，提交成功清空表单，前端不保存已输入令牌。

登记的 Base URL 为服务根地址，标准化末尾斜杠后拼 `/mcp`。API 配置 MCP_ALLOWED_BASE_URLS，默认只显式允许 `http://mcp-demo:8090`；部署者可加入获准精确 scheme/host/port/path 目标。另用 MCP_ALLOWED_TOOLS 按目标列出可执行的只读工具名，默认仅自建服务的 formula_hint、unit_convert，发现不等于获准执行。拒绝凭据、query、fragment、非 HTTP(S)、非白名单目标和重定向；不把“禁止所有私网”当成 SSRF 防护，以免阻断本课同网服务。DNS 解析/建连仍须排除元数据地址和未获准 loopback/link-local 目标，特殊内网目标只可由部署配置授权。

### 3. 数据模型、原子发现与版本授权

新增以下表，外键及查询始终带 class_id：

| 表 | 主要信息与约束 |
| --- | --- |
| mcp_servers | 班级、名称、Base URL、服务端 token、connected/disconnected、连接 revision、协议版本、脱敏错误、时间；班内名称唯一 |
| mcp_tools | server_id、名称、完整说明、inputSchema、规范化声明 hash、发现版本；同服务器工具名唯一 |
| mcp_tool_approvals | skill_id、server_id、tool_name、批准的声明 hash、教师和时间；保存技能时事务替换 |
| mcp_call_audits | class/user/conversation/question、可空 server_id、引用/服务器名快照、call_id、round、状态、限长摘要/原因、时间和耗时；call_id 唯一 |
| mcp_conversation_usage | conversation_id 和累计已受理调用尝试数，按会话唯一，重启后不归零 |

另为 tutor_messages 增加可空 tool_calls_json，用于本人回放，旧行按空集合读取。迁移通过迁移记录或 information_schema 检查保证 ALTER 幂等，不能仅把 ALTER 丢进每次启动都会执行的建表列表。为描述变更增加独立 mcp_tool_changes 记录（服务器名快照、工具名、旧/新声明及 hash、时间），首次发现、变化和删除都记录版本，只向本班教师展示；删除服务器保留变更与调用证据，server_id 置空，不级联删除历史。断开清缓存后重连仍以历史版本比对，不丢失声明变化证据。

connect 在有限超时内握手及完整发现后验证：数量/总大小上限、无重复名、名称可引用、说明有效、JSON Schema 可编译；有分页时遍历至结束，拒绝循环 cursor。先在内存完成所有结果，再在事务中按连接 revision 比较并整体替换；失败不发布部分发现。失败状态改 disconnected，保留的旧成功快照不作为有效缓存；断开明确删除可用缓存，更新名称/地址/token 同样失效。

单实例每个服务器的管理操作和“实际发出调用”共享短锁。网络发现不持数据库事务；revision 防止较慢 connect 在 disconnect 后重新写回 connected。断开返回前不允许新调用进入发送阶段；已发送请求尽力取消，无法撤销远端已完成动作。运行期使用当前状态与 hash 核对，绝不每次重新握手发现。

skill 原文继续保留字符串引用。保存时解析格式、校验班级/连接/工具/只读批准范围，并在同一事务写入技能和批准 hash。重新发现后描述/结构变化使 hash 不匹配，调用 denied；教师查看变化并显式重新保存相关技能表示重新批准。服务器改名/删除不批量改写技能原文；教师在界面修复引用。相比自动继承授权，这能防止变更工具说明悄然扩大权限。

### 4. 管理 API 合约

| 接口 | 成功及主要返回 |
| --- | --- |
| GET/POST /api/mcp/servers | 200 servers / 201 server；含 id/name/base_url/has_token/status/last_error |
| GET/PUT/DELETE /api/mcp/servers/{id} | 200 / 200 / 204；PUT 名称地址必填，token 可省略 |
| POST /api/mcp/servers/{id}/connect | 200 server 和发现清单；失败 503 并 disconnected |
| POST /api/mcp/servers/{id}/disconnect | 200 server，幂等断开 |
| GET /api/mcp/servers/{id}/tools | 200 tools，含完整描述、input_schema、hash，断开时为空 |
| GET /api/mcp/servers/{id}/changes | 200 changes，教师核对工具说明/结构变化，按时间倒序有限分页 |
| GET /api/mcp/audits | 200 audits，limit 默认 20、上限 100，按 (created_at,id) 游标分页 |

统一 401、学生 403、跨班/未知同形 404、输入/重名 400、可重试服务故障 503。变化记录和工具列表继承服务器的归属校验。audit 可按 server_id、状态筛选，过滤器不能绕过 class_id。学生会话内部调用 MCP 无需获得管理接口权限；授权来自本班智能体的当前技能批准集合。

### 5. 工具网关适配与真实最终流

新建独立工具对话网关类型，支持 tools、tool_choice、assistant tool_calls 与 tool 角色，不改变旧 HistoryTurn/StreamAnswer 的公开语义。模型别名采用请求内短名 t1、t2，映射表记录真实 ref 和完整声明 hash；工具说明加来源标签，但不附 Base URL、令牌或服务端错误堆栈。

有有效 MCP 候选时，工具决策阶段使用非流式 `/chat/completions`，tools 来自快照，tool_choice:auto；每次得到结构化 tool_calls，宿主收集完整 arguments、检查 ID/别名/对象、校验后按返回顺序执行，再追加 assistant/tool 配对消息。可连续两轮以上；无调用时结束决策阶段。中间模型正文仅作内部决策内容，不当成最终答案发出或保存。

随后禁用工具并用完整当轮轨迹进行一次 `stream:true` 最终回答，继续用现有引用清理器逐段 flush。不能把非流式决策答案拆片冒充最终流。该方案增加一次最终生成请求，但明确保证所有调用进度在首片正文之前，也避免上游在同一轮混发正文与工具参数造成顺序冲突。工具决策响应允许正文为空且含有效 tool_calls；截断、畸形或异常响应才按协议错误处理，最终流仍要求有效非空正文。网关不支持 function calling 时明确 error，不以固定关键词手工调用替代。

无有效 MCP 候选时保持原 StreamAnswer 直通；技能含已失效引用时，先形成 denied 元信息，传给最终生成的只含固定失败数据，不重新赋予模型失效工具。内置 knowledge_search 与无工具技能均保留第五课的前置绑定材料检索，无额外工具计费记录，不改变旧无命中处理。

### 6. 调用预算、局部失败与信任边界

默认 MCP_MAX_ROUNDS=3、MCP_MAX_CALLS_PER_MESSAGE=6、MCP_MAX_CALLS_PER_CONVERSATION=24；部署配置只能使用正数及受限上界。每轮最多受理 16 个模型提议，超出当轮声明大小/数量边界视为网关协议错误。每次已受理尝试（包括参数拒绝、连接失效和远端失败）先以事务领取持久配额，单次问题和会话分别计数；配额耗尽后的剩余提议只返回 quota denied，不再领取或执行。即使找不到服务器，仍消耗本会话的有效尝试预算，不依赖能否插入服务器审计计数。

单次 MCP 请求超时 5 秒，工具阶段总预算 30 秒，整轮对话共享最长 110 秒 context，为最终生成保留时间；不能让每次重试重新拿 110 秒。达轮次/调用/时间上限后关闭工具并要求基于已有资料和结果作答，工具失败为关联 tool 消息，局部故障不终止正文生成；客户端取消立即停止后续调度，取消网关及远端请求。

使用项目内固定版本的 JSON Schema 校验库（支持 Draft 2020-12），关闭网络/文件 `$ref` 自动解析，禁止外部引用。缓存声明必须能编译；对不支持的结构明确连接失败，不忽略关键约束。参数校验与模型声明同源，服务端拒绝附加字段和越权别名。既有模板继续限定用户/班级和材料，不向工具发送完整历史，仅传模型提出且通过字段约束的学习问题参数。

远端响应最多 64 KiB，提取文本并按最多 2000 字送入模型；摘要最多 200 字。空/非文本/畸形响应、自报 isError 为 error。返回在进模型前检查已知指令式载荷，命中整条替换固定说明并记 error。数据通过带来源的 tool 消息回传，不能进入 system；此检查降低已知注入风险，真正动作约束仍为候选集、hash 授权、Schema、出站白名单和预算。本课允许的工具仅是部署者确认的只读提示工具，写入/外发类型默认拒绝，未来引入应另行规划人工确认流程。

### 7. SSE、历史与审计各司其职

先发原 meta(skill/citations)，工具决策每轮完成后再发含该轮新增 tool_calls 的 meta，三项状态和脱敏 text 不含原始参数。每项 call_id 为服务端生成的稳定事件标识，模型 tool_call_id 仅用于网关消息配对，避免模型伪造导致审计去重错误。后续 meta 重带一致的 skill/citations，不变动引用编号；无调用省略 tool_calls。工具阶段保持定期 SSE 注释心跳以免代理超时，不增加新的业务事件名。

前端按 call_id 合并到当前助手消息，后续 meta 不覆盖已显示调用。发送期间继续禁用切换/重复提交；离开和停止 abort。最终助手保存含聚合 tool_calls_json，刷新回放本人元信息；只有工具动作没有正文时不写助手消息，审计仍保留已发生动作。内部 assistant/tool 轨迹只属于当轮网关上下文，不进入六条持久用户/助手历史。

能定位本班服务器则每次尝试写一条 ok/error/denied；无法归属的失效引用仅留本次元信息，不落到别班。审计先建立带 call_id 的尝试记录，执行后更新终态；意外退出遗留未终结记录在恢复时标为 error/结果未知，不能假称未执行。无法可靠写入尝试审计时拒绝发出远端动作；远端已执行但审计收尾失败则报告 error/结果未知，不假称动作已回滚。数据库整体不可用依旧可能触发原会话 error，不把基础存储故障包装成正常工具降级。

### 8. 增量界面、文档与验收

沿用 TutorSettings 新增 MCP 分区，列服务器、密码输入、连接/断开、完整声明与变更、审计游标；保存技能前展示引用所对应声明和版本。TutorChat 在引用旁显示逐轮调用状态，摘要按纯文本渲染。沿用主题、移动布局与键盘交互，不重做现有页面。

扩展 handler 注解、共享 schemas.json、生成 openapi.json 和 Swagger 示例，不手改生成物；补充本课 docs/LESSON-6.md 与验证记录。apitest 增加独立第六课 Compose 项目/测试卷及真实 mcp-demo，沿用 mock 模型但让其提出真实结构化调用；测试用 MCP token 从独立环境生成，不读取当前私人 .env。

至少覆盖两工具直连、登记发现/重连替换、技能原子验证、声明变更再批准、两轮调用、同轮多调用、参数拒绝、越权、断开/删除、超时/isError、预算、注入、取消、元信息回放与数据库副作用。跑旧第三至第五课回归、Swagger 试调和浏览器首片时序。真实模型与同伴交叉核验单独记录，缺少则保持未完成，不混入 mock 通过结论。

## Risks / Trade-offs

- [课程页面对连接失败状态叙述不同] → 采用任务清单及实践页的 disconnected 结果；不发布失败清单，旧快照不得授权调用。
- [任务清单写用户消息，课件指定 tool 消息] → 按课件补充要求选 tool 角色，并在 proposal 说明；保持数据侧边界。
- [同伴示例说“自己的服务器”，当前应用按班授权] → 按任务清单与现有模型以班级为管理边界；同伴隔离用他班账号验证，同班教师共享管理，不悄然引入个人租户。
- [unit_convert 名称像计算器] → 说明、输出和真实模型验收都坚持单位关系/步骤提示，不返回题目最终数值。
- [发现后远端实际行为仍可能改变] → hash 批准只能约束可见声明，配合只读信任来源、运行期授权、出站限制和预算，不宣称消除所有投毒风险。
- [管理操作与远端执行存在时间差] → revision/短锁阻止断开后新发请求；对已经发出的动作只尽力取消并如实审计。
- [MODIFIED 尚无主规约基线] → 依赖第五课 delta，未来先同步前置，再同步本变更；严格结构校验通过不代表当前就能独立归档。

## Migration Plan

1. 用户后续发出实施指令后再修改代码，先核对工作区已有改动及前置基线，不自动归档第五课。
2. 实施新增表及消息元信息幂等迁移，在隔离测试库连续运行两次，保留原账号、材料、技能、消息和索引。
3. 增加内部 Compose 服务、白名单及预算环境变量示例，示例不包含可用秘密；登记/连接由教师正常表单完成。
4. 完成独立服务、接口/数据库、单元、构建、生成文档和浏览器验收；分别记录 mock、真实模型、同伴证据与未完成项。
5. 回滚应用可保留新增表/列；旧字段仍可读，停用新增服务即可退出 MCP 路径，不删历史、不清卷。旧版解析器不能重新保存 MCP 引用技能，该兼容限制在回滚说明中显式告知。
6. 同步/归档需用户另行指令且先处理前置主规约；生产部署、主分支合并和远程资源操作另行授权。

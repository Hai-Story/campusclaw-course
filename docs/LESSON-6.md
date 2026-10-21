# 第六课：MCP 与工具接入

本课在第五课智能体、技能和私有流式会话上增加同仓 MCP 提示服务。实现依据是 `openspec/changes/add-mcp-tools-integration/`。真实模型与同伴验收的状态见 [验证记录](LESSON-6-VALIDATION.md)。

## 启动与连接

1. 在本机环境变量文件中自行设置独立的 `MCP_AUTH_TOKEN`。不要使用应用登录 Token、网关密钥或示例占位值。
2. 正常启动 `docker compose up --build -d`。服务 `mcp-demo:8090` 只在 Compose 内网提供接口，没有宿主端口映射；API 不依赖其启动健康状态，MCP 离线不影响无工具的旧功能。
3. 教师进入「助手配置 → 本班工具服务」，登记引用名 `campus-tools`，Base URL 填 `http://mcp-demo:8090`，密码框输入与服务环境变量相同的令牌。服务端读取只返回 `has_token`，更新时留空保留旧令牌，勾选清除才删除它。
4. 点击「连接并发现」，核对完整说明、参数结构与版本。默认提供 `formula_hint` 和 `unit_convert`，分别给步骤检查和单位换算方向，不计算整题最终数值。
5. 在技能工具字段填写下例引用，点击「刷新可用工具声明」查看当前内容，再保存技能。将技能绑定给本班智能体，学生选该智能体创建会话后提问。

```text
名称：MCP数学解题引导
适用条件：去括号，单位，换算
工具：knowledge_search，formula_hint@campus-tools，unit_convert@campus-tools
启用：是

先判断学生当前步骤，给检查方向。只给学习提示，不报整题最终得数。
```

内置 `knowledge_search` 和空工具技能继续使用第五课前置材料检索，资料范围仍仅限智能体绑定的本班材料。MCP 候选由命中技能决定，模型选择是否调用以及参数；列在技能里不表示每次都会调用全部工具。

## 连接、版本和授权

教师管理接口全部要求应用 Bearer；学生连只读管理操作也返回 403。班级从服务端认证会话取得；跨班编号与未知编号同形 404。MCP 服务另用自己的 Bearer，二者不能互换。

连接经过 initialize、initialized 通知、完整 tools/list，再整体替换本地工具缓存。连续连接不会复制工具。失败进入 disconnected，不使用旧快照授权；断开清空可用缓存。改名、改地址或改令牌也使连接失效，须重新连接；改名不会自动改写技能里的字符串引用。

工具说明或 inputSchema 变化会留下版本记录，并使技能已批准的 hash 失效。教师核对新声明后重新保存技能才恢复执行。断开/删除后旧技能保留原文，但调用明确显示 denied。已发出的请求会尽力取消，远端已完成动作无法撤销。

默认只允许部署者列出的只读提示工具。工具返回、说明与上传材料都是不可信数据；返回经已知指令载荷检查后作为 tool 消息回传，不能进入 system。句式检查不是完整的注入防护，候选集、当前授权、参数校验、出站白名单与配额仍由宿主执行。

## 配置

| 变量 | 默认值/含义 |
| --- | --- |
| MCP_AUTH_TOKEN | 自建服务启动必填；仅运行时注入 |
| MCP_ALLOWED_BASE_URLS | `http://mcp-demo:8090`；逗号分隔的精确获准 Base URL |
| MCP_ALLOWED_TOOLS | 空值使用自建服务的两个提示工具；可设 JSON，例如 `{"http://mcp-demo:8090":["formula_hint","unit_convert"]}` |
| MCP_MAX_ROUNDS | 每问最多 3 轮工具决策，允许范围 1–10 |
| MCP_MAX_CALLS_PER_MESSAGE | 每问最多受理 6 次尝试，范围 1–32 |
| MCP_MAX_CALLS_PER_CONVERSATION | 每会话累计最多受理 24 次，范围 1–1000；重启和下一次提问不重置 |

单次 MCP 请求最多 5 秒，工具阶段最多 30 秒，整轮生成共享 110 秒。已受理的拒绝和失败也计次数；预算外剩余提议显示 denied，关闭后续工具决策并用已有信息继续作答。局部工具错误不单独中断生成，网关或基础数据库错误仍按会话 error 收尾。

Base URL 不接受凭据、查询、片段、重定向或白名单外目标；元数据/link-local 目标拒绝。默认 Compose 内网地址可用。需要测试其他服务时须由部署者显式扩展目标与逐工具白名单，不能仅凭远端 readOnlyHint 自动赋权。Schema 使用固定版本校验库，禁止外部 `$ref` 加载。

服务端数据库的 MCP token 字段属于秘密存储；备份和数据库访问也需限制。前端、API 返回、日志、提示词、调用审计与报告不得包含令牌原文。本课没有引入独立密钥管理系统。

## 协议和流式行为

MCP 固定兼容 `2025-06-18` 的无状态 HTTP JSON 子集，支持 POST /mcp、初始化通知、工具发现和调用，GET /mcp 返回 405。工具服务 /health 公开。宿主发送版本头，接受有限大小 JSON 或 POST SSE 单响应。不支持 stdio、资源/提示模板、服务端通知通道、会话恢复或通用写入工具。

有工具时先进行非流式结构化决策，可多轮调用，再关闭工具并单独请求真正的 `stream:true` 最终生成。中间决策正文不作为最终答案拆片输出；无 MCP 路径沿用旧的直接流式生成。

```text
event: meta
data: {"status":"ok","skill":{"id":1,"name":"MCP数学解题引导"},"citations":[]}

event: meta
data: {"status":"ok","skill":{"id":1,"name":"MCP数学解题引导"},"citations":[],"tool_calls":[{"ref":"formula_hint@campus-tools","status":"ok","text":"请逐项核对系数。","round":1,"call_id":"example-call"}]}

event: delta
data: {"text":"先检查括号内每一项。"}

event: done
data: {"message_id":2}
```

首帧仍带 skill/citations；每轮调用再补发 meta，所有调用事件早于最终正文。无调用省略 tool_calls。前端按 call_id 合并，重复 meta 不清空原调用；工具阶段可有 SSE 注释心跳。完整/部分助手消息保存当轮调用元信息，刷新可回放；只有工具动作而没有正文时不伪造助手消息，已发生动作保留审计。

## 管理 API 和审计

| 方法与路径 | 含义 |
| --- | --- |
| GET/POST /api/mcp/servers | 列表、登记（201） |
| GET/PUT/DELETE /api/mcp/servers/{id} | 详情、修改、删除（204） |
| POST /api/mcp/servers/{id}/connect | 握手和发现 |
| POST /api/mcp/servers/{id}/disconnect | 幂等断开并清缓存 |
| GET /api/mcp/servers/{id}/tools | 已连接的完整工具声明 |
| GET /api/mcp/servers/{id}/changes | 发现与声明变更，limit/before 分页 |
| GET /api/mcp/audits | 本班调用审计，limit/before/status/server_id 过滤 |

审计记录引用、服务器名快照、用户/会话/提问 ID、轮次、耗时、时间及 ok/error/denied 和限长摘要，不返回完整学生会话或原始工具参数。无法归属的已删除引用只在应答显示 denied；已发生的历史审计不会随服务器删除。教师仍无法打开学生本人私有会话。

审计分页游标由接口返回，客户端原样回传，默认 20、最多 100。Swagger 与 OpenAPI 由原有处理器注解和 schemas.json 生成，新增令牌输入标 writeOnly，响应仅 has_token。

## 验证

```sh
cd backend
go test ./...
go test -race ./internal/mcp ./internal/tutor ./internal/knowledge
go run ./cmd/openapi --check
cd ../mcp-server
go test ./...
cd ../frontend
npm test
npm run build
cd ..
openspec validate add-mcp-tools-integration --strict
```

依赖只安装在项目内。当前会话实际使用的工具、已跑检查与限制见验证记录。

Docker 可用后执行：

```sh
sh apitest/run_lesson6.sh
# Docker Hub 不可用且本机已有所需公开基础镜像、Go/npm 依赖时：
APITEST_CACHED_IMAGES=1 sh apitest/run_lesson6.sh
# 旧入口 apitest/run_local.sh 也转到第六课隔离验收。
# 需要真实浏览器检查时：
APITEST_BROWSER=1 sh apitest/run_lesson6.sh
```

脚本使用单独的 campusclaw-lesson6 Compose 项目、数据库和卷，首次生成 `.tools/lesson6-test.env` 测试凭据，不读取应用私人 `.env`。真实 MCP 与模拟模型配合验证协议链路；逐接口脚本检查响应与数据库，HTML 报告保存在 Git 忽略的 apitest/reports/。环境不足或缺必需用例非零退出，不算通过。

缓存镜像路径使用公开 golang:1.22-alpine 编译并运行本次源码、nginx:stable-alpine 提供 Web；不使用旧应用二进制。它验证本次代码的容器链路，不能代替生产 Dockerfile 基础镜像的标准构建。

`apitest/browser_mcp_ui.py` 是独立前端 API fixture 检查脚本，不等于数据库/真实链路验收；当前沙箱不允许 Chromium 启动。正式浏览器端到端仍需运行接入真实隔离环境的脚本。

## 数据与回滚

迁移新增六张 MCP 表，以及消息 tool_calls_json。启动先检查列是否存在，再做增量 ALTER，不覆盖旧用户、技能、材料或消息。进程异常遗留的 pending 审计在下一次单实例启动标记为“结果未知”，不得假称动作未执行。

旧版程序可保留新表/列运行旧路径，但旧版技能解析器不能重新保存含 MCP 引用的技能。回滚时先停用 MCP 技能或恢复其内置工具版本，保留审计与消息历史，不删表、不清卷。

OpenSpec 三份 MODIFIED delta 依赖尚未同步的第五课基线。未来先处理前置规约，再按指令同步/归档本变更；本次不自动部署、同步或归档，Git 交付按用户指令执行。

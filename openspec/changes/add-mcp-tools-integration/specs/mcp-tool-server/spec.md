## Purpose

为 CampusClaw 提供与主应用同仓、可独立启动和验收的真实 MCP 工具服务，使宿主通过带令牌的协议发现并执行学习提示工具，验证远端通信而非仅验证宿主模拟逻辑。

## ADDED Requirements

### Requirement: Same-repository authenticated MCP service

系统 MUST 在 `mcp-server/` 提供独立服务，由现有 Compose 一并构建启动并与 API 在内部网络互通。服务 MUST 提供公开 `GET /health` 和 Bearer 鉴权的 `POST /mcp`；令牌 MUST 从 `MCP_AUTH_TOKEN` 环境变量读取，缺失时启动失败，无令牌或错误令牌返回 401。令牌 MUST 不进入 Git、镜像层、前端或日志，Compose MUST 不默认发布 MCP 宿主端口。

#### Scenario: Real service is reachable only with credentials

- **WHEN** 从同网 API 容器检查健康、无令牌调用以及带正确令牌调用
- **THEN** 健康返回 200，无令牌与错误令牌均为 401，正确令牌可完成发现和调用；该结果来自自建服务

### Requirement: Negotiated JSON-RPC tool protocol

服务 MUST 实现 JSON-RPC 2.0 的 `initialize`、`notifications/initialized`、`tools/list`、`tools/call`，正确关联请求/响应 ID，声明 tools 能力和约定的协议版本。通知接受后 MUST 返回无正文的 202；非法 JSON、无效请求、未知方法、错误参数 MUST 返回可读且不含秘密的协议错误，不将协议失败伪装成成功。HTTP 行为 MUST 明确支持无状态 JSON 响应的课程子集，不声称支持资源、提示模板、会话恢复或服务端推送；不支持的 GET 流返回 405。

#### Scenario: Initialize and discover tools

- **WHEN** 宿主协商受支持版本、发送 initialized 通知并执行 tools/list
- **THEN** 收到一致版本、tools 能力、通知确认以及含名称、完整说明、对象 inputSchema 的工具清单

#### Scenario: Protocol failures remain distinguishable

- **WHEN** 提交畸形 JSON、未知方法、错误参数或请求不存在的工具
- **THEN** 返回相应 JSON-RPC 错误及可用请求 ID；工具业务执行失败用 `isError:true` 表达，宿主可区分协议失败和业务失败

### Requirement: At least two discoverable guidance tools

服务 MUST 至少提供 `formula_hint` 与 `unit_convert` 两个工具，名称符合 `^[a-z][a-z0-9_]{0,47}$`。前者给公式/步骤检查方向，后者给单位关系/换算步骤提示；两者 MUST 返回文本内容及 `isError`，不得直接完成整题或给最终数值答案，不得执行文件访问、命令、数据写入或转发到其他外部服务。说明 MUST 写清适用场景、参数、单位与提示边界；参数 MUST 符合公开的 inputSchema。

#### Scenario: Both tools have independently verifiable behavior

- **WHEN** 带令牌分别发现并调用两个工具，包含有效和无效参数
- **THEN** 能发现相应声明；有效参数返回相关学习提示，无效参数被拒；结果不包含整题最终得数，不产生外部副作用

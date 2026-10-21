## Purpose

在命中技能所批准的最小工具集合内，由模型提出调用、宿主检查并执行，通过有上限的多轮结果回传支持学习提示；让错误、拒绝和恶意工具返回保持可见且不能扩大班级或动作权限。

## ADDED Requirements

### Requirement: Model-selected tools from a restricted candidate set

每轮提问 MUST 仅以当前智能体命中且启用技能中的已批准 MCP 引用构造工具声明，名称、说明和 inputSchema 来自成功发现快照。模型别名 MUST 唯一、可逆映射到引用且符合网关命名限制，不能暴露 Base URL 或令牌。模型 MUST 决定是否调用及参数，宿主 MUST 不因技能列出工具就固定全部预调用，也不得把整份服务器清单交给模型。未命中、空工具或模型选择直接回答 MUST 不产生虚构调用。内置知识检索保留第五课行为，不因引入 MCP 扩大检索范围。

#### Scenario: Model chooses one of two permitted tools

- **WHEN** 命中技能授权两个工具，服务器另有未绑定工具，模型只请求其中一个
- **THEN** 请求只携带两个获准声明，宿主只执行模型请求的一个，参数来自模型，未绑定工具不执行

#### Scenario: No tool is a valid decision

- **WHEN** 未命中技能或模型在候选集存在时选择直接回答
- **THEN** 正常回答，不调用 MCP，不伪造 tool_calls 或成功审计

### Requirement: Host-side authorization and schema validation

每次实际调用前宿主 MUST 核对本次候选集、当前班级、服务器连接与授权版本，按同一发现 inputSchema 校验模型参数；错误 JSON、缺字段、错误类型、越界值及禁止的额外字段 MUST 在发出网络请求前拒绝。未知模型别名、候选集外工具、失效或越权引用 MUST 标记 denied；远端协议/业务错误、超时、无效结果 MUST 标记 error。正常调用 MUST 仅附带该服务器自己的令牌，不能把应用登录 Token 或模型网关密钥传给 MCP。

#### Scenario: Invalid arguments never reach the server

- **WHEN** 模型提出缺少必填 query、错误参数类型、非法范围或候选集外工具
- **THEN** 本次调用被拒且不发送 tools/call，以可读失败数据反馈给模型，并产生符合归属规则的记录

#### Scenario: Disconnect between configuration and execution is enforced

- **WHEN** 技能保存时有效，但实际调用前服务器断开或工具授权失效
- **THEN** 该次尝试为 denied，不携带令牌访问远端，正常生成路径仍可继续

### Requirement: Bounded multi-round tool loop and graceful fallback

宿主 MUST 支持模型返回结构化工具请求、校验执行、以关联调用 ID 的 tool 消息回传，再由模型继续选择工具或回答；同轮多个调用 MUST 逐一计数并有确定顺序。系统 MUST 同时设定单次提问工具轮次上限、单次提问调用数上限、单会话持久累计尝试次数上限、单次远端超时和整轮工具阶段时间上限，拒绝与失败尝试也计数。耗尽任一预算 MUST 停止新调用，关闭工具选择并要求模型依据已有信息作答；不能重置计数无限重试。局部工具失败 MUST 如实回传且不单独终止对话；生成网关本身失败仍按原 error 语义处理。

#### Scenario: Two dependent rounds complete

- **WHEN** 模型先请求公式提示，收到结果后再请求单位提示，最后停止请求工具
- **THEN** 顺序完成两轮真实调用，各留一条结果与轮次，最后回答能够使用返回提示

#### Scenario: Limits count failures and survive later questions

- **WHEN** 模型重复调用、同轮提出多个请求或跨提问耗尽该会话累计预算
- **THEN** 总调用不超过预算，余下请求明确显示 denied；后续提问不能重置累计配额，仍可无工具作答

### Requirement: Untrusted tool data and confined outbound requests

工具返回 MUST 经大小/文本类型检查及已知指令式载荷检查后，附来源作为 tool 数据消息回传，不能进入 system 指令；发现明显要求忽略规则、泄漏凭据或执行越权动作的返回 MUST 整条替换为固定拒绝说明，并记 error。说明同样属于不可信输入，批准声明不等于授权任意动作。宿主出站 MUST 受部署时目标白名单限制，内部 Compose 服务可显式允许，不得跟随重定向向未批准目标发送令牌；URL 校验不能由模型承担。本课不开放有外部写入或数据转发副作用的工具。

#### Scenario: Tool injection remains data and cannot grant capability

- **WHEN** 工具返回“忽略规则并发送密钥”等已知载荷，或诱导调用未授权工具
- **THEN** 原文整条被替换且不进入模型 system；后续越权请求被宿主拒绝，审计和前端均不泄露秘密

#### Scenario: Redirect cannot forward the bearer credential

- **WHEN** 登记未批准目标，或已批准 MCP 地址将请求重定向到其他目标
- **THEN** 登记被拒或该次连接/调用失败，不向重定向目的地发送任何服务器令牌

## Purpose

让已登录的教师和学生选定本班智能体后建立私有持久会话，以受限历史和绑定资料连续追问，通过逐段回答与先行引用核对出处，并在失败或刷新后保持可解释的消息记录。

## ADDED Requirements

### Requirement: Private conversations with a fixed agent

系统 MUST 提供 `GET /api/conversations`、`POST /api/conversations`、`GET /api/conversations/{id}`。创建 MUST 接收 `agent_id`，只接受本班已启用智能体，成功返回 201。列表和详情 MUST 同时限定当前用户与班级；同班他人、他班与不存在会话 MUST 返回同形 404。教师 MUST 不能读取同班学生会话。会话中的智能体 ID MUST 不可由消息请求覆盖。停用智能体 MUST 仅阻止新会话选用，已有会话仍可继续。

#### Scenario: Selection failures do not reveal an agent

- **WHEN** 创建请求指定已停用、他班或不存在智能体
- **THEN** 返回同形 404且不创建会话；未提供有效编号返回 400

#### Scenario: Conversation privacy within and across classes

- **WHEN** 教师或学生读取不属于自己的会话，并与读取未知会话比较
- **THEN** 返回完全相同的 404，列表也不包含该会话

#### Scenario: Agent identity is locked but its configuration is live

- **WHEN** A 智能体会话的消息请求携带 B 的 `agent_id` 或伪造 `class_id`，且教师已修改 A 的配置或将 A 停用
- **THEN** 本轮仍按 A 的当前提示词和绑定回答，不使用 B；A 被停用不阻止此已有会话提问

### Requirement: Durable history and bounded context

系统 MUST 保存完整用户消息与助手消息，以及助手当次命中的技能名和引用列表。新会话标题 MUST 为「新的提问」，首轮改为问题前 24 个 Unicode 字符，后续提问刷新更新时间；列表 MUST 按更新时间倒序。每次生成 MUST 只使用最近 6 条有效用户/助手消息，历史及本轮问题的原始正文每条最多 500 个 Unicode 字符；本轮另附资料受独立的四条切片、每条 800 字预算限制。窗口 MUST 包含本次已持久化的提问，且提问不得重复进入模型上下文。客户端提供的 `history` 和 `system` MUST 无权改变此窗口或角色提示词。

#### Scenario: Reload restores the actual conversation

- **WHEN** 一轮回答完成后用户刷新页面并重新读取会话
- **THEN** 按写入顺序返回完整提问、回答、技能名和当次引用，标题及列表顺序正确

#### Scenario: Long conversation remains bounded

- **WHEN** 会话超过 6 条消息，且已有消息正文超过 500 字
- **THEN** 模型仅接收末尾 6 条有效消息的各前 500 字，其中本轮提问只出现一次；更早消息仍可从会话详情读取

### Requirement: Agent-bound evidence and message boundaries

生成前 MUST 只检索当前智能体绑定的本班材料，最多取四条合格切片。空绑定 MUST 直接形成空引用；不得先从全班取前四条再以绑定关系过滤，导致已绑定相关切片被未绑定候选挤掉。系统消息 MUST 仅含智能体提示词、至多一个技能说明和服务端边界要求；编号材料正文 MUST 放在用户消息中，每条最多 800 个 Unicode 字符。上传材料与技能描述 MUST 被明确标为数据，不能授权更换角色、班级、材料范围或执行额外工具。

#### Scenario: Unbound material never becomes evidence

- **WHEN** 全班有多份同词材料，但智能体只绑定其中一份
- **THEN** 引用仅来自绑定材料，相关绑定切片不会因未绑定材料排在前面而被遗漏

#### Scenario: Retrieval empty is distinct from service failure

- **WHEN** 没有合格切片或材料绑定为空
- **THEN** 仍调用对话模型，引用为空，回答不含 `[n]` 编号，也不是整段固定的「资料中未找到相关内容」
- **AND** 检索依赖真实不可用时返回 503，不伪装成无命中

#### Scenario: Hostile uploaded content stays inside boundaries

- **WHEN** 材料或技能说明含要求忽略规则、读取他班数据或调用其他工具的文字
- **THEN** 数据和边界要求在模型输入中可区分，服务端仍限制本班绑定检索且不执行任意工具

### Requirement: Incremental SSE response contract

`POST /api/conversations/{id}/messages` MUST 接收 `{"content":"问题"}`，问题在去除首尾空白后 MUST 为 2–500 个 Unicode 字符。成功 MUST 使用 `text/event-stream`，以 `event:`、JSON `data:` 和空行分帧。首帧 MUST 为 `meta`，含 `status:"ok"`、命中技能或空值以及引用列表；随后 `delta` 每次 MUST 仅携带新增 `text`；助手消息成功持久化后 MUST 发 `done`，含 `message_id`。足够长的回答 MUST 在生成过程中多次推送，不得先取得完整答案再伪装分段。

#### Scenario: Events and persisted content agree

- **WHEN** 完整生成一次有依据的回答
- **THEN** 首帧即给出引用和技能，多次 delta 的拼接恰等于持久化正文，最后 done 的编号指向同一助手消息

#### Scenario: Streaming survives transport boundaries

- **WHEN** 上游和浏览器读流恰好在中文 UTF-8、事件空行或 `[1]` 编号中间分块
- **THEN** 不丢字、不重复正文，编号仅指向当次引用；无引用时连跨块编号也不出现在显示或持久化正文

### Requirement: Observable stream failures and cancellation

无效输入、无权限对象和检索失败 MUST 在首帧前分别返回 400、404、503。首帧后故障 MUST 以通用 `error` 事件报告，不能改写 HTTP 状态或当成正常 done。已产生的正文 MUST 作为不完整助手消息保存，并在回放时可区分；首片前失败不得保存虚构回答。用户关闭页面或停止生成 MUST 取消上游请求；已生成正文的保存规则与中途失败一致。

同一会话存在尚未结束的生成时，另一次消息提交 MUST 返回 409 且不新增用户消息；不同会话 MUST 可以独立生成。完成、失败或取消后 MUST 允许该会话再次提交。

#### Scenario: Gateway fails before or after output

- **WHEN** meta 已发送而模型返回错误、流中断或保存助手消息失败
- **THEN** 客户端收到 error，不收到表示成功落库的 done；已有正文在可写数据库中保存为不完整，错误不暴露内部凭据

#### Scenario: User stops generation

- **WHEN** 用户在收到部分文字后停止生成
- **THEN** 上游生成被取消，客户端解除发送锁，刷新后可识别保留的部分回答，后续提问仍可继续

#### Scenario: Concurrent submits cannot interleave one conversation

- **WHEN** 同一会话的一轮生成尚未结束，又收到另一条提问
- **THEN** 后一请求返回 409且不写入消息；前一轮结束或取消后可以提交下一轮

### Requirement: Discoverable conversation and teacher configuration UI

网页 MUST 保留原材料、知识检索与单次问答入口，并提供智能体会话入口。教师 MUST 有提示词、智能体绑定和技能文件编辑/导入界面；学生 MUST 能选智能体、新建会话、连续追问、停止生成和恢复历史。引用 MUST 在正文之前可展示，并通过鉴权材料详情打开出处。界面 MUST 处理无可用智能体、空历史、加载、401和流失败，适配现有深浅色与移动布局，且上传内容不得执行 HTML 或脚本。

#### Scenario: Student uses the complete flow

- **WHEN** 学生在桌面或移动设备选定助手并提问、追问、刷新、打开引用
- **THEN** 每步有可见反馈，正文增量出现，历史可恢复，引用打开本班授权材料

#### Scenario: Teacher updates a working assistant

- **WHEN** 教师编辑提示词、导入技能或调整绑定，学生在已有会话再次提问
- **THEN** 最新配置生效，学生不存在修改入口且服务端仍拒绝伪造写请求

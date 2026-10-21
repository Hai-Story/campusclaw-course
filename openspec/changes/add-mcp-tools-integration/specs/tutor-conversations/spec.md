## MODIFIED Requirements

### Requirement: Incremental SSE response contract

`POST /api/conversations/{id}/messages` MUST 接收 `{"content":"问题"}`，去首尾空白后 MUST 为 2–500 个 Unicode 字符。成功 MUST 使用 `text/event-stream`，以 `event:`、JSON `data:` 和空行分帧。首帧 MUST 为 `meta`，含 `status:"ok"`、命中技能或空值以及引用列表；执行 MCP 时 MUST 在最终正文首片前按轮补发 `meta`，每轮含该轮新增的 `tool_calls`。每项 MUST 含 `ref`、`status:ok/error/denied`、`round`、`call_id` 和脱敏限长的 `text`；无调用的 meta MUST 省略 tool_calls。meta 中 skill/citations MUST 保持一致，前端累积调用且以 call_id 去重，不能用后续 meta 清空既有调用或正文。delta MUST 只含新增 text；只有完整助手正文持久化后 MUST 发含 message_id 的 done。足够长的回答 MUST 在生成过程中多次推送，不得取得完整最终答案后拆片冒充流式。

#### Scenario: Events and persisted content agree

- **WHEN** 完整生成一次有依据的回答，过程中模型完成两轮工具调用
- **THEN** 首帧给出技能与引用，两轮工具 meta 顺序到达且先于正文；delta 拼接等于持久化正文，done 指向同一助手消息

#### Scenario: Streaming survives transport boundaries

- **WHEN** 上游和浏览器恰好在中文 UTF-8、事件空行、工具参数片段或引用编号中间分块
- **THEN** 不丢字、不重复正文，工具参数完整组装后才执行；无引用时跨块引用编号也不会出现在显示或持久化正文

#### Scenario: No calls keep the previous event shape

- **WHEN** 本轮没有 MCP 调用，或模型在有候选工具时直接回答
- **THEN** 仍为 meta/delta/done，meta 不含 tool_calls，既有 skill/citations 字段与旧界面语义兼容

### Requirement: Discoverable conversation and teacher configuration UI

网页 MUST 保留原材料、知识检索与单次问答入口，并提供智能体会话入口。教师 MUST 有提示词、智能体绑定和技能文件编辑/导入界面，并能进入 MCP 服务器管理、查看工具完整声明、连接/断开及查看本班调用记录。学生 MUST 能选智能体、新建会话、连续追问、停止生成和恢复历史；无教师管理入口，服务端仍拒绝伪造管理请求。引用与本次工具调用状态 MUST 在正文之前可展示，引用仍通过鉴权材料详情打开。界面 MUST 区分 ok/error/denied，处理无助手、空历史、加载、401和流失败，适配现有深浅色与移动布局，上传内容及工具文本 MUST 不执行 HTML 或脚本。教师审计仅显示必要调用摘要，不开放读取学生完整会话。

#### Scenario: Student uses the complete flow

- **WHEN** 学生在桌面或移动设备选助手，提问并触发工具，再追问、停止、刷新或打开引用
- **THEN** 工具状态先于正文，正文真实增量、历史可恢复、引用仅打开授权材料；取消与失败仍保留原有不完整消息语义

#### Scenario: Teacher updates a working assistant

- **WHEN** 教师导入带 MCP 引用的技能或断开其服务器，学生在已有会话再次提问
- **THEN** 最新配置生效；断开调用显示 denied 且可在本班审计核对，学生无法操作管理接口

## ADDED Requirements

### Requirement: Durable tool metadata without exposing conversation internals

本人会话回放 MUST 返回每条助手消息对应的脱敏工具调用元信息，不能因刷新丢失工具来源。只有调用而尚无助手正文的中断 MUST 不创建虚构助手消息，但已发生的可归属动作 MUST 留在审计。MCP 往返的 assistant tool_calls 与 tool 消息 MUST 不作为普通对话正文进入最近六条历史窗口，不得挤掉用户/助手有效历史。取消 MUST 传播到网关与远端调用，停止后不得继续新工具调用；本班他人及跨班仍无法读取本人会话。

#### Scenario: Reload and partial completion remain truthful

- **WHEN** 工具调用成功后助手部分输出，再因用户取消而中断并刷新
- **THEN** 已有正文以 incomplete 回放，关联调用可见；原同会话锁释放，可继续提问；无正文时不伪造助手回答

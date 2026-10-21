## Purpose

让教师以第五课的中文字段文件维护本班技能，保存并取回原文，按照问题确定性选择当前智能体绑定的单个技能，并使解题引导与技能开关具有可验证的回答差异。

## ADDED Requirements

### Requirement: Validated course skill files

系统 MUST 通过技能文件原文解析名称、适用条件、资料、工具、启用和空行后的说明。原文 MUST 是有效 UTF-8 且不超过 8000 个 Unicode 字符；名称为 1–32 字、适用条件为 1–120 字、说明为 1–2000 字、可选资料范围最多 32 字。名称、适用条件和启用 MUST 必填；启用 MUST 为「是」或「否」；工具只允许 `knowledge_search`，未填写时工具列表为空。元数据注释 MUST 允许以 `#` 开头。缺字段、重复字段、未知字段、非法工具、无正文或超长输入 MUST 返回带可读原因的 400，且不得创建或覆盖技能。

#### Scenario: Valid source can be round-tripped

- **WHEN** 教师提交符合课程格式的技能文件
- **THEN** 系统保存解析字段与原文，读取后可取回同一原文继续编辑

#### Scenario: Invalid file is rejected without residue

- **WHEN** 文件缺少必填项、重复名称字段、包含未知元数据或工具 `shell`、缺正文、超长或不是有效 UTF-8
- **THEN** 系统返回具体可读的 400 错误，不留下新技能，也不改变待修改技能

### Requirement: Class-scoped skill lifecycle

系统 MUST 提供 `GET /api/skills`、`GET /api/skills/{id}`、`POST /api/skills`、`PUT /api/skills/{id}` 和 `DELETE /api/skills/{id}`。创建和改写 MUST 接收 `{"source":"文件原文"}`；创建返回 201、改写返回 200、删除返回 204。所有接口 MUST 要求 Bearer，所有写操作 MUST 仅允许教师；班内名称 MUST 唯一，重名返回 400。按编号操作他班与不存在技能 MUST 返回同形 404。教师 MUST 能以改写文件中的启用值启停技能，删除 MUST 同时清除智能体绑定。

#### Scenario: Student lists but cannot write skills

- **WHEN** 学生列出本班技能，随后尝试创建、修改、停用或删除
- **THEN** 列表不含他班技能，所有写请求返回 403 且无数据变更

#### Scenario: Same-class uniqueness and hidden object lookup

- **WHEN** 教师提交本班同名技能，或按编号访问他班与未知技能
- **THEN** 同名请求返回 400；他班与未知编号返回完全相同的 404

### Requirement: Deterministic single-skill routing

每轮会话回答 MUST 仅从所选智能体当前绑定且启用的本班技能中路由，至多命中一个。适用条件 MUST 按中英文逗号、顿号、分号或竖线分词，忽略空关键词并在匹配前统一大小写。问题包含一个关键词计一分；最高分胜出，同分取技能 ID 较小者；全部零分 MUST 不选技能。未绑定、停用或已删除的技能 MUST 不参与。

#### Scenario: Tie and case folding

- **WHEN** 问题包含英文小写关键词，与两项大小写不同但得分相同的有效技能匹配
- **THEN** 仅选中 ID 较小的一项，不注入两份技能说明

#### Scenario: Binding and enabled status take effect immediately

- **WHEN** 教师停用、删除或解绑原本命中的技能，学生在已有会话重问同一句
- **THEN** 该技能不再参与路由，无需新建会话

### Requirement: Guidance behavior and subject scope

提供的示例与验收配置 MUST 至少包含一项名称含「解题引导」的有效技能。命中此类技能时，回答 MUST 指出已有步骤是否正确及下一步检查方向，不直接给最终得数或完成整题；停用或解绑后，同一问题 MUST 回到智能体直接回答策略。命中技能的资料范围非空时，引用 MUST 只保留标题包含该范围文本的已绑定材料，且筛选后重新连续编号。

#### Scenario: Guidance differs from ordinary answering

- **WHEN** 学生询问技能适用词对应的题目和错误步骤，随后教师停用该技能并让学生重问
- **THEN** 启用时给出检查方向而不报最终答案；停用后按智能体提示词直接回答，有证据时两次引用都与材料一致

#### Scenario: Subject mismatch removes evidence

- **WHEN** 命中数学资料范围技能，但某个检索候选的材料标题不含数学
- **THEN** 该候选不进入用户消息或引用列表，剩余引用从 1 连续编号；若全部被筛去仍继续无引用回答

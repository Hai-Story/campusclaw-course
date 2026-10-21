## MODIFIED Requirements

### Requirement: Validated course skill files

系统 MUST 通过技能文件原文解析名称、适用条件、资料、工具、启用和空行后的说明。原文 MUST 是有效 UTF-8 且不超过 8000 个 Unicode 字符；名称为 1–32 字、适用条件为 1–120 字、说明为 1–2000 字、可选资料范围最多 32 字。名称、适用条件和启用 MUST 必填；启用 MUST 为「是」或「否」。工具 MUST 允许内置 `knowledge_search` 及 `工具名@服务器名`，可用中英文逗号、顿号、分号或竖线分隔，去空白并按首次出现去重，未填写时为空。MCP 工具名 MUST 符合 `^[a-z][a-z0-9_]{0,47}$`，服务器名 MUST 符合 `^[a-z][a-z0-9_-]{0,31}$`；裸工具名、大写工具名和工具名中的连字符 MUST 被拒绝。元数据注释 MUST 允许以 `#` 开头。缺字段、重复字段、未知字段、非法引用、无正文或超长输入 MUST 返回可读的 400，且不得创建或覆盖技能。

#### Scenario: Valid source can be round-tripped

- **WHEN** 教师提交符合课程格式且混合内置工具及有效 MCP 引用的技能文件
- **THEN** 系统保存解析字段与原文，读取可取回同一原文继续编辑，工具集合按首次出现的顺序去重

#### Scenario: Invalid file is rejected without residue

- **WHEN** 文件缺必填项、重复字段、含未知字段、含 shell 或裸 formula_hint、含 Formula_Hint@campus-tools 或 formula-hint@campus-tools、缺正文、超长或非有效 UTF-8
- **THEN** 返回可读的 400，不留下新技能，也不改变旧技能及其工具授权

## ADDED Requirements

### Requirement: Class-scoped MCP reference validation on save

创建与更新技能时系统 MUST 对每个 MCP 引用验证当前会话班级内存在该名称服务器、服务器 connected、工具在本次发现清单中且结构受支持；教师 MUST 能在提交前查看该工具完整声明。验证通过 MUST 绑定该版声明的授权；任一引用失败 MUST 返回可读的 400，并整体保留旧技能和旧授权。引用他班服务器与不存在服务器 MUST 使用相同错误，不泄露他班存在性。运行期引用失效 MUST 不自动删除技能原文或回退为成功；停用、删除、解绑技能仍沿用第五课规则。

#### Scenario: A mixed tool list commits atomically

- **WHEN** 技能包含一个有效引用和一个未知、他班、断开或清单外引用
- **THEN** 整份技能保存被拒，原文、解析字段、智能体绑定及授权记录不发生部分更新

#### Scenario: Existing knowledge-only skills stay valid

- **WHEN** 教师读取或保存原先只含 knowledge_search 或空工具的有效技能
- **THEN** 保持原先文件格式、启停、路由计分及学科筛选行为，无需登记 MCP 服务器

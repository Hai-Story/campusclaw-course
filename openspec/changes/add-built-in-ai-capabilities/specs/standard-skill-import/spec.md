## Purpose

在保留课程中文字段技能文件的基础上，支持带 YAML frontmatter 的单文件 SKILL.md，使教师能够预览、配置触发和工具并保存原文，通过现有智能体路由使用技能而不执行上传代码。

## ADDED Requirements

### Requirement: Dual format import and preview
系统 SHALL 保持旧中文字段文件及 source-only 接口兼容，支持标准文件的 name、description 与非空 Markdown 正文，原文 UTF-8 且最多 8000 字，标准名称最多 64 字。标准正文不沿用旧说明 2000 字限制，非法 YAML、重复字段、缺字段与超限返回 400 且不写库。预览 SHALL 返回解析结果与格式。

#### Scenario: Standard file round trip
- **WHEN** 教师导入合法 SKILL.md 并配置 routing 后保存
- **THEN** 可取回完全相同原文、独立触发配置及解析说明，长名称能随回答写入历史

#### Scenario: Reject invalid file
- **WHEN** 文件缺元数据、无正文、YAML 畸形或超长
- **THEN** 预览或保存返回明确 400，不新增或覆盖技能

### Requirement: Explicit skill routing and authority
标准技能保存 SHALL 要求 routing 的 trigger、subject、tools、enabled，遵守现有长度、工具引用、班级、MCP 当前版本批准规则；文件其他元数据不得授予工具或脚本权限。教师 SHALL 能编辑原文、配置、启停、删除和绑定。

#### Scenario: Uploaded tool declaration is not authority
- **WHEN** frontmatter 声明工具但 routing.tools 为空
- **THEN** 不授予或调用该工具，保存仍以教师明确配置为准

#### Scenario: Approved standard skill calls MCP
- **WHEN** 教师连接发现工具，核对并保存标准技能的工具引用，再绑定助手提问
- **THEN** 模型可选择调用，显示调用进度并留存审计；断开、超时、声明变更或权限不足受到现有约束

### Requirement: Built in skills
系统 SHALL 按班级幂等预置数学解题引导、语文阅读分析、英语学习指导，重启不覆盖教师修改、停用或删除，不自动批准 MCP。

#### Scenario: Initial skill list
- **WHEN** 新班级初始化完成
- **THEN** 三项技能可被助手绑定且工具不包含未批准的外部引用

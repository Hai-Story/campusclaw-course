## Why

现有系统具备智能体、知识检索与 MCP 链路，但新班级没有默认应用与技能，提示词不能作为模板复用，技能导入只识别课程中文字段。补齐默认内容与标准指令文件导入，使五项能力可直接使用和验收。

## What Changes

- 按班级初始化三个技能、三个只读提示词模板及三个已绑定技能的学习助手；保留教师修改与删除结果。
- 增加本班提示词模板管理和套用，已有助手保留套用时的正文副本。
- 兼容 YAML frontmatter 的单文件 SKILL.md，原文保留，触发、资料、工具与开关由教师显式配置。
- 验证预置助手的知识范围、引用及标准技能的 HTTP MCP 批准、调用与审计，完善操作提示。
- 五项能力各提交一次，最后 README 与验证记录单独提交，推送 implement。

## Capabilities

### New Capabilities
- `built-in-ai-apps`: 班级预置助手、幂等初始化与应用选择。
- `prompt-templates`: 内置模板、本班自定义模板和复制套用。
- `standard-skill-import`: 双格式导入、原文、显式路由配置和预置技能。

### Modified Capabilities
无独立主规约可修改。现有 tutor-skills、tutor-agents、tutor-conversations 来自 add-tutor-agents-skills-streaming-chat，在途 MCP 行为来自 add-mcp-tools-integration；新规格增量扩展这些实现，不归档或同步前置变更。知识库与 MCP 原有要求保持，集成验收列于新能力场景和 tasks。

## Impact

影响 Go db/tutor、React 助手配置及会话页面、OpenAPI 与接口/浏览器验收。新增 YAML 解析依赖、模板与初始化记录表，扩容技能名和历史技能名到 64 字。无生产部署，无任意脚本或 stdio 支持。

## Context

参见 proposal.md。代码已具备 db/tutor、班级检索、HTTP MCP 和隔离 Docker 验收。主 specs 为空，前置教学与 MCP 规约仍在途，本变更不替代或归档它们。

## Goals / Non-Goals

增量补齐默认内容、模板和标准单文件技能，继续使用现有鉴权和路由。不执行上传脚本、不加载附属文件、不扩展传输协议。

## Decisions

- 默认内容在 backend/internal/catalog 静态定义，db 按 classes 全量遍历，通过 builtin_initializations(class_id,version) 的唯一键在事务内领取初始化版本；成功后再也不自动恢复删除、改名、停用内容。同名用户对象跳过，不自动绑定。技能、模板、应用分别有独立版本，便于五次能力提交。
- 新表 tutor_prompt_templates(id,class_id,name,prompt,builtin)，班内名称唯一。内置模板只读；自定义模板教师可写。GET/POST /api/prompt-templates 与 PUT/DELETE /api/prompt-templates/{id}。跨班与不存在同形 404，学生 403，内置修改/删除 403。套用仅复制文字到助手编辑器。
- 使用 YAML 库解析 frontmatter 的 name/description；其他元数据不转为工具或运行权限。技能原文不变，解析后的说明由 description 与正文组成。标准格式 POST/PUT /api/skills 使用 source 加 routing:{trigger,subject,tools,enabled}；课程文件仍只有 source。新增 POST /api/skills/preview 进行同源服务端解析，返回名称、说明、格式；标准文件预览不要求 routing，保存必须明确提供。
- 课程格式保留旧校验，标准技能名最多 64 字、源文件最多 8000 字。name 与 tutor_messages.skill_name 增量扩至 64，防止长技能名历史落库失败。解析格式由原文判定，无需另存格式列。
- 三个助手从静态模板获取初始 prompt，仅新建时绑定由本次初始化产生且未删除的默认技能；默认材料为空。卡片复用现有创建会话接口。
- 内置模板与技能正文可进入模型上下文，但上传内容不扩大角色、班级、材料和工具范围；MCP 始终由宿主批准、参数校验与配额约束。

## Risks / Trade-offs

- YAML 文件可能依赖外部资源 → 界面说明仅导入当前文件，元数据不执行；非法/重复 YAML 字段报错。
- 名称冲突或默认技能已被教师改动 → 不改用户已有对象，不重建绑定；新应用仍能无资料对话。
- 模拟模型无法证明语义质量 → 自动验收覆盖链路，真实模型另列待验证任务。

- MySQL JSON 会对检索分数 double 产生末位舍入 → 验收仅对 keyword_score/vector_score 使用 1e-12 浮点容差，引用原文、ID、位置及字段集仍严格比较。

## Migration Plan

在原幂等迁移后检查信息表，按需扩容两列并创建新增表；启动后执行按班级版本初始化。模板与初始化记录是增量，保留旧数据。回滚可保留新增表和列；旧程序无法保存标准技能，需先停用相关技能。

## Purpose

为教师和学生提供可直接选用的数学、语文与英语学习助手，默认绑定角色与学科技能，避免新班级必须先手动配置才能开始学习对话，同时保留已有班级和教师自主配置。

## ADDED Requirements

### Requirement: Ready to use class assistants
系统 SHALL 为每个现有及新增班级预置数学解题助手、语文阅读助手、英语学习助手，各自启用并绑定对应预置技能，默认不绑定材料或外部服务。用户 SHALL 可从卡片选择助手并创建本人会话。

#### Scenario: Start without setup
- **WHEN** 新班级用户进入解题会话并选择预置应用
- **THEN** 可立即创建会话和无引用回答，无需先上传资料或登记 MCP

### Requirement: Preserve initialized content
系统 SHALL 按班级记录初始化版本，重启不得重复创建或恢复已修改、停用、删除的内容，同名用户对象不得被覆盖或自动改变绑定。

#### Scenario: Restart after teacher changes
- **WHEN** 教师修改并停用助手、删除技能后重启
- **THEN** 用户修改保持，删除技能不恢复，助手数量不增加

### Requirement: Bound knowledge evidence
预置应用 SHALL 沿用本班绑定材料范围检索和引用，默认空绑定时仍回答；未绑定和跨班材料不得进入模型证据。

#### Scenario: Evidence is scoped
- **WHEN** 三个应用分别绑定本班材料并提问
- **THEN** 有依据时返回正确出处，无依据时无引用，其他材料不进入证据

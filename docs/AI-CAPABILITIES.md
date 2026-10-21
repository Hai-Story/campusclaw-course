# 五项 AI 能力操作说明

## 为预置应用绑定知识库

1. 教师在材料库上传 UTF-8 `.txt` 或 `.md`，等待索引就绪。
2. 在“助手配置”选择数学解题助手、语文阅读助手或英语学习助手，勾选本班材料并保存。
3. 默认技能分别限定标题中的“数学”“语文”“英语”，材料标题应包含对应学科；也可由教师编辑技能的资料范围。
4. 学生选择应用并提问。引用显示材料和切片，点击可打开原文；未绑定、跨班或不符合技能资料范围的材料不会进入引用。
5. 不绑定材料也能对话，此时没有资料引用。已有会话下一轮读取最新绑定，无需重新创建。

材料检索支持关键词、向量与混合三种模式。向量和混合依赖嵌入网关与 Qdrant；依赖故障会明确报错，不伪装成“没有资料”。

复现预置应用的隔离集成验收：

```sh
sh apitest/with_lesson6_env.sh .venv/bin/python apitest/verify_builtin_apps.py
sh apitest/with_lesson6_env.sh .venv/bin/python apitest/verify_builtin_knowledge.py
```

上述脚本只接受显式授权的本机 `campusclaw_test_` 数据库，并使用测试模型；不能代表真实模型的语义质量。

## 标准技能接入 MCP

1. 部署者允许服务根地址和只读工具。默认同仓服务为 `http://mcp-demo:8090`，提供 `formula_hint` 与 `unit_convert`；其他服务须加入 `MCP_ALLOWED_BASE_URLS` 和 `MCP_ALLOWED_TOOLS`。
2. 教师在“本班工具服务”保存根地址与独立 MCP Bearer 令牌，点击“连接并发现”，核对工具说明和参数。应用登录 Token 不能替代 MCP 令牌。
3. 导入含 `name`、`description` frontmatter 与非空正文的 SKILL.md，预览后填写触发关键词、资料范围、工具引用和启用状态。
4. 工具引用写为 `formula_hint@服务器引用名` 等；核对声明后保存技能即批准当前版本，再将技能绑定助手。
5. 模型只在问题匹配技能时获得候选工具，自行决定调用。对话显示轮次、成功/出错/被拒，教师可查询本班审计。
6. 重新发现后若声明变化，旧批准失效。教师须查看差异并重新保存相关技能；断开、删除或修改服务会使旧连接失效。

文件里的 `allowed-tools` 等附加元数据不会授予权限，上传文件不运行脚本、不安装依赖。当前仅支持 HTTP MCP 工具子集，不支持 stdio、资源、提示模板或会话恢复。

```sh
sh apitest/with_lesson6_env.sh .venv/bin/python apitest/verify_standard_mcp.py
```

该验收使用真实独立 MCP 服务和测试专用可控代理，覆盖两轮调用、声明变更、重新批准、超时、断开、停用、权限与审计；模型决策来自模拟网关。

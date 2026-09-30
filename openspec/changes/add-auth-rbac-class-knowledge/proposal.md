# Change: add-auth-rbac-class-knowledge

## Why

CampusClaw 后续的检索、备课与学情能力都依赖可信的身份、班级数据边界和已入库的教研材料。本变更先建立可复现、可负向验收的最小底座。

## What Changes

- 增加账号密码登录、数据库可撤销会话与签名 Bearer Token，并支持立即失效的登出。
- 增加教师/学生 RBAC；教师可上传和删除本班材料，学生只读。
- 以会话中的 `class_id` 强制班级隔离，跨班与不存在资源返回同形 404。
- 支持 `.txt`/`.md` 上传、正文解析、材料与知识库同事务入库及鉴权下载。
- 教师可在确认后删除本班上传或预置材料；删除预置材料的决定持久保存，重启不会自动补回。
- 增加登录页和材料页，包括主题、双视图、本班搜索、命令面板、进度与安全 Markdown 展示。
- 以 Nginx、Go、MySQL 和 Docker Compose 交付单实例版本。

## Non-goals

- 不做知识库问答、向量检索、RAG 或跨班检索；本迭代只保存可查正文。
- 不做对话助手、作业布置/批改、成绩和错题本。
- 不做 OAuth、SSO、注册、改密、验证码或邮箱短信登录。
- 不做平台超级管理员；跨班运维与审计留待隔离稳定后的独立变更。
- 不做 PDF、Word、图片解析、在线预览或编辑。
- 不做多校多租户、Kubernetes、CI、公网域名、HTTPS 和多副本高可用。

## Impact

- 新增 `backend/`、`frontend/`、Nginx 与 Compose 配置。
- 新增 `classes`、`users`、`sessions`、`materials`、`knowledge_entries` 五张表。
- 对外只暴露 Web 端口；数据库和上传目录不直接暴露。

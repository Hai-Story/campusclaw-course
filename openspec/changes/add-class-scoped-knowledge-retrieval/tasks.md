## 1. Data model and configuration

- [ ] 1.1 Add repeatable MySQL migrations for `knowledge_chunks` with ngram FULLTEXT and `knowledge_index_jobs`, class indexes and versioned IDs; verify a second migration run leaves data unchanged.
- [ ] 1.2 Add validated server-only embedding/dialogue gateway configuration, model/version, vector dimension, Qdrant endpoint, query limits and retry policy; verify missing or mismatched values fail startup and secrets never reach the frontend.
- [ ] 1.3 Add internal-only Qdrant with persistent vector volume to Compose; verify `docker compose config` shows only `web` publishing a host port.

## 2. Index creation and recovery

- [ ] 2.1 Implement Unicode-codepoint `auto` (800/80), bounded `custom` and Markdown `hierarchy` splitting with offsets and offset basis; verify boundaries, preprocessing and unchanged original body in unit tests.
- [ ] 2.2 Insert a versioned pending index job in the teacher-upload transaction while keeping its 201 response; verify successful upload creates one job and a rolled-back upload leaves no job, material, knowledge row or file.
- [ ] 2.3 Implement idempotent auto backfill for existing entries; verify repeated backfill creates no duplicate jobs and changes no source body or material ID.
- [ ] 2.4 Implement worker leasing, gateway embedding, MySQL chunk upsert, Qdrant point upsert with point ID equal to chunk ID, retry/backoff and ready/failed transitions; verify restart after vector upsert produces no duplicate visible chunks.
- [ ] 2.5 Add teacher-only same-class reindex with selected strategy and operator retry/rebuild for failed jobs or index-version changes; verify old chunks/vectors disappear and original material remains unchanged.

## 3. Authorized retrieval and answer APIs

- [ ] 3.1 Add authenticated `GET /api/knowledge/search` with trimmed 1–200-codepoint `q`, default 10/max 20 `limit`, `keyword|vector|hybrid` modes and 400/401 responses; verify boundary inputs in handler tests.
- [ ] 3.2 Implement class-filtered MySQL ngram keyword ranking over ready chunks without gateway/Qdrant calls; verify Chinese exact-term hits and continued keyword service during vector outage.
- [ ] 3.3 Query Qdrant with server-session class and active version, reject cosine below 0.35, then hydrate through class-filtered MySQL rows; verify forged `class_id`, stale point and cross-class candidates never leak content.
- [ ] 3.4 Fuse independently filtered paths with RRF k=60 for hybrid mode; verify a two-path hit receives both contributions and vector/hybrid return 503 on dependency outage.
- [ ] 3.5 Expose `ready|building|degraded` class index state and fixed no-evidence message; verify no-match 200, pending indexing, failed indexing and service outage remain distinct.
- [ ] 3.6 Return chunk ID/index, title, filename, MySQL excerpt, `[start,end)` offsets and basis without storage paths or vectors; verify every excerpt equals stored chunk text.
- [ ] 3.7 Add authenticated `POST /api/ask` using top four hybrid hits and bounded history, with fixed empty answer and zero dialogue calls on no hit; verify forged system/class input is ignored.
- [ ] 3.8 Call dialogue gateway only with authorized numbered evidence and validate answer citation numbers; verify `[1]`/`[2]` map to returned citations and invalid numbers cannot leak through.

## 4. Web search and source navigation

- [x] 4.1 Add separately discoverable knowledge-search and knowledge-answer views, with three search modes and an ask input, without replacing `/api/materials?q=...`; verify material-list search and command palette still work.
- [ ] 4.2 Render hits, source metadata, citations, no-evidence/pending/degraded/error states and bounded input as safe text; verify uploaded HTML cannot execute in results or answers.
- [ ] 4.3 Open authenticated material detail from a hit, highlight original-basis range or label processed-basis excerpt, and retain download; verify stale or cross-class navigation shows unavailable.
- [ ] 4.4 Cancel or ignore out-of-order search/ask responses when the query changes; verify an earlier slow request cannot replace current results.

## 5. Integration and documentation

- [ ] 5.1 Extend A/B integration checks for three modes, forged class, cross-class/unknown 404 equality, unauthenticated 401, safe snippets and no-evidence answer without dialogue calls; verify the script against Compose.
- [ ] 5.2 Verify seed and new uploads become searchable, custom/hierarchy rebuild replaces old chunks, and interrupted indexing resumes without duplicates; verify against Compose with a test gateway.
- [ ] 5.3 Document gateway data flow/configuration, model/version pinning, index states, rebuild, rollback and unchanged material search in README; verify documented clean-volume commands where the environment supports them.
- [ ] 5.4 Run Go tests/vet/build, frontend production build and `openspec validate add-class-scoped-knowledge-retrieval --strict`; record actual outcomes and any unavailable environment checks before marking implementation complete.
- [x] 5.5 Verify `course-embedding` (2048 dimensions) and `course-chat` through the authenticated API; when chat omits inline markers, show labelled retrieved sources while rejecting out-of-range citations.
- [x] 5.6 Verify deleting uploaded and seed materials removes their search/ask candidates and vector points without worker resurrection.

## 6. Knowledge-answer chatbot refinement

- [x] 6.1 Keep “知识问答” in desktop left and mobile navigation; implement consecutive chat messages, multiline input, Unicode limits, IME-safe keyboard submission and per-answer source buttons.
- [x] 6.2 Send at most six completed history messages of 1000 codepoints each through authenticated `/api/ask`; return and display class index state while retaining `.env` server-only model configuration and evidence-gated answers.
- [x] 6.3 Implement pending/stop/retry/clear states; preserve completed chats across navigation, cancel inactive/unmounted requests and ignore late responses without duplicate messages.
- [x] 6.4 Verify history, no-evidence/index warnings, request failures, cancellation, sources, navigation, safe rendering, desktop/mobile and existing material/search behavior; run frontend tests/build, relevant backend checks and strict OpenSpec validation, and document actual environment limitations.

### Chatbot validation record (2026-10-01)

- Worktree branch: `l4-implement`. 本轮更新 proposal/design/spec 并实现 6.1–6.4，未依据本轮模拟验证勾选前面既有的完整 Compose 验收项；change 尚未归档。
- `frontend`: `npm ci`、`npm test`（6 项测试）和 `npm run build` 均通过。
- `backend`: 项目 `.tools` 中 Go 1.22.12 的 `go test -ldflags=-linkmode=external ./...`、`go vet ./...` 和 `CGO_ENABLED=0 GOOS=linux go build ./cmd/server` 均通过。普通内部链接测试在当前 macOS 报 `missing LC_UUID load command`，使用已有系统 clang 的外部链接后所有包通过；工具链和缓存均位于项目中。
- `scripts/verify_chatbot.py`: 项目 `.venv` 内 Playwright、临时独立的 headless Chrome 上下文、16 次模拟 ask 请求全部通过。覆盖最近六条历史、200 个 Unicode 字符、中文输入法/换行、连续消息、每轮来源和原文高亮、无依据/索引提示、失败重试、停止后草稿不误发、清空/切换/登出后的迟到响应、刷新/重新登录清空聊天、安全 Markdown、材料搜索/三种检索/命令面板、深浅主题、390px 与 320px 手机宽度（发送按钮可见且无横向溢出）。截图在忽略目录 `output/playwright/`。
- `openspec validate add-class-scoped-knowledge-retrieval --strict` 与 `git diff --check` 通过。
- 当前 worktree 未复制 `.env`。只检查主项目 `.env` 变量名，使用 `docker compose --env-file <主项目 .env> config --quiet` 验证配置通过，未输出密钥或口令。Docker daemon 当前未运行，真实模型网关、MySQL、Qdrant 和 Compose 端到端联调未执行；模拟浏览器验收和本地网关单元测试不能替代此项。
- Go 依赖直连下载超时后，确认 `127.0.0.1:7897` 代理可用，仅对验证命令设置 `HTTP_PROXY`/`HTTPS_PROXY` 重试。未修改全局代理、系统权限或生产部署。

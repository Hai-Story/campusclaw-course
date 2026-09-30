import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { ApiError, ChunkStrategy, KnowledgeHit, Material, User, api, downloadMaterial, uploadMaterial, SearchMode } from "./api";

type Theme = "light" | "dark";
type ViewMode = "list" | "grid";
type Toast = { id: number; message: string; tone: "success" | "error" | "info" };

function App() {
  const [status, setStatus] = useState<"loading" | "guest" | "ready">("loading");
  const [user, setUser] = useState<User | null>(null);
  const [theme, setTheme] = useState<Theme>(() =>
    localStorage.getItem("campus-theme") === "dark" ? "dark" : "light"
  );

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem("campus-theme", theme);
  }, [theme]);

  useEffect(() => {
    api.me()
      .then(({ user: current }) => {
        setUser(current);
        setStatus("ready");
      })
      .catch(() => setStatus("guest"));
  }, []);

  const unauthorized = useCallback(() => {
    setUser(null);
    setStatus("guest");
  }, []);

  if (status === "loading") return <LoadingScreen />;
  if (status === "guest") {
    return (
      <LoginScreen
        theme={theme}
        onTheme={() => setTheme((value) => (value === "light" ? "dark" : "light"))}
        onLogin={(current) => {
          setUser(current);
          setStatus("ready");
        }}
      />
    );
  }
  return (
    <Library
      user={user!}
      theme={theme}
      setTheme={setTheme}
      onUnauthorized={unauthorized}
    />
  );
}

function LoadingScreen() {
  return (
    <main className="loading-screen" aria-live="polite">
      <div className="seal">C</div>
      <div className="loading-line" />
      <p>正在确认教研仓身份…</p>
    </main>
  );
}

function LoginScreen({
  theme,
  onTheme,
  onLogin
}: {
  theme: Theme;
  onTheme: () => void;
  onLogin: (user: User) => void;
}) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const result = await api.login(username, password);
      onLogin(result.user);
    } catch (reason) {
      setError(reason instanceof ApiError ? reason.message : "登录未完成");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="login-shell">
      <button className="theme-fab" onClick={onTheme} aria-label="切换深浅色主题">
        {theme === "light" ? "夜" : "昼"}
      </button>
      <section className="login-story" aria-label="CampusClaw 介绍">
        <div className="campus-gate" aria-hidden="true">
          <span /><span /><span /><span /><span />
        </div>
        <div className="story-copy">
          <p className="story-mark">CampusClaw 教研仓</p>
          <h1>让每份教研材料，<br />留在正确的班级里。</h1>
          <p>身份、权限与知识入库，从这里成为后续智能教学的可信底座。</p>
        </div>
      </section>
      <section className="login-panel">
        <div className="login-card">
          <div className="seal">C</div>
          <div>
            <h2>进入教研仓</h2>
            <p className="muted">请使用课程预置的教师或学生账号</p>
          </div>
          <form onSubmit={submit}>
            <label>
              账号
              <input
                autoFocus
                autoComplete="username"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                placeholder="例如 teacher_a"
                required
              />
            </label>
            <label>
              密码
              <input
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="输入账号密码"
                required
              />
            </label>
            {error && <p className="form-error" role="alert">{error}</p>}
            <button className="primary wide" disabled={busy}>
              {busy ? "正在确认…" : "登录"}
            </button>
          </form>
          <p className="privacy-note">登录状态保存在 HttpOnly 会话中，不在浏览器保存角色或班级。</p>
        </div>
      </section>
    </main>
  );
}

function Library({
  user,
  theme,
  setTheme,
  onUnauthorized
}: {
  user: User;
  theme: Theme;
  setTheme: (theme: Theme) => void;
  onUnauthorized: () => void;
}) {
  const [materials, setMaterials] = useState<Material[]>([]);
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<Material | null>(null);
  const [selectedCitation, setSelectedCitation] = useState<KnowledgeHit | null>(null);
  const [section, setSection] = useState<"materials" | "knowledge">("materials");
  const [view, setView] = useState<ViewMode>(() =>
    localStorage.getItem("campus-view") === "grid" ? "grid" : "list"
  );
  const [uploadOpen, setUploadOpen] = useState(false);
  const [commandsOpen, setCommandsOpen] = useState(false);
  const [toasts, setToasts] = useState<Toast[]>([]);
  const searchRef = useRef<HTMLInputElement>(null);

  const toast = useCallback((message: string, tone: Toast["tone"] = "info") => {
    const id = Date.now() + Math.random();
    setToasts((items) => [...items, { id, message, tone }]);
    window.setTimeout(() => setToasts((items) => items.filter((item) => item.id !== id)), 3600);
  }, []);

  const handleError = useCallback((reason: unknown) => {
    if (reason instanceof ApiError && reason.status === 401) {
      onUnauthorized();
      return;
    }
    toast(reason instanceof Error ? reason.message : "请求未完成", "error");
  }, [onUnauthorized, toast]);

  const loadMaterials = useCallback(async (search: string) => {
    setLoading(true);
    try {
      const result = await api.materials(search);
      setMaterials(result.materials);
    } catch (reason) {
      handleError(reason);
    } finally {
      setLoading(false);
    }
  }, [handleError]);

  useEffect(() => {
    const timer = window.setTimeout(() => loadMaterials(query), 220);
    return () => window.clearTimeout(timer);
  }, [query, loadMaterials]);

  useEffect(() => {
    localStorage.setItem("campus-view", view);
  }, [view]);

  useEffect(() => {
    const listener = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setCommandsOpen((value) => !value);
      }
      if (event.key === "Escape") {
        setCommandsOpen(false);
        setUploadOpen(false);
        setSelected(null);
      }
    };
    window.addEventListener("keydown", listener);
    return () => window.removeEventListener("keydown", listener);
  }, []);

  async function openMaterial(item: Material, citation: KnowledgeHit | null = null) {
    try {
      const result = await api.material(item.id);
      setSelected(result.material);
      setSelectedCitation(citation);
    } catch (reason) {
      handleError(reason);
    }
  }

  async function openCitation(hit: KnowledgeHit) {
    await openMaterial({ id: hit.material_id } as Material, hit);
  }

  async function logout() {
    try {
      await api.logout();
    } finally {
      onUnauthorized();
    }
  }

  const commands = useMemo(() => [
    { label: "搜索本班材料", hint: "/", run: () => { setSection("materials"); window.setTimeout(() => searchRef.current?.focus(), 0); } },
    { label: "知识库检索", hint: "检索", run: () => setSection("knowledge") },
    ...(user.role === "teacher" ? [{ label: "上传教研材料", hint: "教师", run: () => setUploadOpen(true) }] : []),
    { label: theme === "light" ? "切换到深色主题" : "切换到浅色主题", hint: "主题", run: () => setTheme(theme === "light" ? "dark" : "light") },
    { label: view === "list" ? "切换到网格视图" : "切换到列表视图", hint: "视图", run: () => setView(view === "list" ? "grid" : "list") },
    { label: "退出当前账号", hint: "会话", run: logout }
  ], [theme, user.role, view]);

  return (
    <div className="app-shell">
      <aside className="rail">
        <div className="brand-lockup">
          <div className="seal small">C</div>
          <div><strong>CampusClaw</strong><span>教研仓</span></div>
        </div>
        <nav aria-label="主导航">
          <button className={`nav-item ${section === "materials" ? "active" : ""}`} onClick={() => setSection("materials")}><span>册</span>材料库</button>
          <button className={`nav-item ${section === "knowledge" ? "active" : ""}`} onClick={() => setSection("knowledge")}><span>⌕</span>知识检索</button>
          <button className="nav-item" onClick={() => setCommandsOpen(true)}><span>⌘</span>快捷命令</button>
        </nav>
        <div className="rail-note">
          <span className="status-dot" />
          <p>当前数据边界</p>
          <strong>{user.class_name}</strong>
          <small>列表、详情与下载均由服务端校验</small>
        </div>
        <div className="profile">
          <div className="avatar">{user.username.slice(0, 1).toUpperCase()}</div>
          <div><strong>{user.username}</strong><span>{user.role === "teacher" ? "教师" : "学生"}</span></div>
          <button onClick={logout} aria-label="退出登录">退出</button>
        </div>
      </aside>

      <main className="workspace">
        <header className="workspace-header">
          <div>
            <p className="location">{user.class_name} / {section === "materials" ? "材料库" : "知识检索"}</p>
            <h1>{section === "materials" ? "教研材料" : "可追溯检索"}</h1>
            <p className="header-note">{section === "materials" ? `共 ${materials.length} 份可见材料` : "检索本班切片，核对每条依据"} · 权限来自服务端会话</p>
          </div>
          <div className="header-actions">
            <button className="icon-button" onClick={() => setTheme(theme === "light" ? "dark" : "light")} aria-label="切换主题">
              {theme === "light" ? "夜" : "昼"}
            </button>
            <button className="command-button" onClick={() => setCommandsOpen(true)}>
              快捷命令 <kbd>⌘ K</kbd>
            </button>
            {user.role === "teacher" && <button className="primary" onClick={() => setUploadOpen(true)}>上传材料</button>}
          </div>
        </header>

        <div className="mobile-sections" role="group" aria-label="工作区切换">
          <button className={section === "materials" ? "selected" : ""} onClick={() => setSection("materials")}>材料库</button>
          <button className={section === "knowledge" ? "selected" : ""} onClick={() => setSection("knowledge")}>知识检索</button>
        </div>

        {section === "materials" ? <><section className="toolbar" aria-label="材料筛选与视图">
          <div className="search-wrap">
            <span aria-hidden="true">⌕</span>
            <input
              ref={searchRef}
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="搜索本班标题或正文"
              aria-label="搜索本班材料"
            />
            {query && <button onClick={() => setQuery("")} aria-label="清空搜索">×</button>}
          </div>
          <div className="view-toggle" aria-label="视图切换">
            <button className={view === "list" ? "selected" : ""} onClick={() => setView("list")} aria-label="列表视图">☷</button>
            <button className={view === "grid" ? "selected" : ""} onClick={() => setView("grid")} aria-label="网格视图">▦</button>
          </div>
        </section>

        <section className={`material-collection ${view}`} aria-live="polite" aria-busy={loading}>
          {loading ? (
            <div className="empty-state"><div className="paper-stack" /><h2>正在取阅材料</h2></div>
          ) : materials.length === 0 ? (
            <div className="empty-state">
              <div className="paper-stack" />
              <h2>{query ? "没有匹配的本班材料" : "本班资料架还是空的"}</h2>
              <p>{query ? "换一个关键词试试。搜索不会跨越班级边界。" : user.role === "teacher" ? "上传第一份 .txt 或 .md 教研材料。" : "等待教师上传教研材料。"}</p>
            </div>
          ) : materials.map((item, index) => (
            <article className="material-item" key={item.id}>
              <button className="material-open" onClick={() => openMaterial(item)} aria-label={`查看 ${item.title}`}>
                <span className="shelf-index">{String(index + 1).padStart(2, "0")}</span>
                <span className={`file-mark ${item.original_name.endsWith(".md") ? "markdown" : "text"}`}>
                  {item.original_name.endsWith(".md") ? "MD" : "TXT"}
                </span>
                <span className="material-copy">
                  <strong>{item.title}</strong>
                  <small>{formatDate(item.created_at)} · {formatBytes(item.size_bytes)} · {item.class_name}</small>
                </span>
              </button>
              <button className="quiet-action" onClick={() => downloadMaterial(item).then(() => toast("下载已开始", "success")).catch(handleError)}>下载</button>
            </article>
          ))}
        </section></> : <KnowledgePanel onOpen={openCitation} onError={handleError} />}
      </main>

      {selected && <DetailDrawer item={selected} citation={selectedCitation} onClose={() => { setSelected(null); setSelectedCitation(null); }} onDownload={() => downloadMaterial(selected).then(() => toast("下载已开始", "success")).catch(handleError)} onReindex={user.role === "teacher" ? async (strategy) => { await api.reindex(selected.id, strategy); toast("索引重建已排队", "success"); } : undefined} />}
      {uploadOpen && user.role === "teacher" && <UploadDialog onClose={() => setUploadOpen(false)} onUploaded={() => { setUploadOpen(false); loadMaterials(query); toast("材料已入库，本班现在可以查看", "success"); }} onError={handleError} />}
      {commandsOpen && <CommandPalette commands={commands} onClose={() => setCommandsOpen(false)} />}
      <div className="toast-region" aria-live="polite">
        {toasts.map((item) => <div className={`toast ${item.tone}`} key={item.id}>{item.message}</div>)}
      </div>
    </div>
  );
}

function KnowledgePanel({ onOpen, onError }: { onOpen: (hit: KnowledgeHit) => void; onError: (reason: unknown) => void }) {
  const [query, setQuery] = useState("");
  const [mode, setMode] = useState<SearchMode>("hybrid");
  const [hits, setHits] = useState<KnowledgeHit[]>([]);
  const [indexState, setIndexState] = useState<"ready" | "building" | "degraded">("ready");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [answer, setAnswer] = useState<{ text: string; citations: KnowledgeHit[] } | null>(null);
  const [asking, setAsking] = useState(false);
  const askController = useRef<AbortController | null>(null);

  useEffect(() => {
    askController.current?.abort();
    setAsking(false);
    setAnswer(null);
    if (!query.trim() || Array.from(query.trim()).length > 200) {
      setHits([]);
      setLoading(false);
      setError("");
      return;
    }
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      setLoading(true);
      setError("");
      api.knowledgeSearch(query.trim(), mode, controller.signal)
        .then((result) => {
          if (controller.signal.aborted) return;
          setHits(result.hits);
          setIndexState(result.index_state);
        })
        .catch((reason) => {
          if (controller.signal.aborted) return;
          setHits([]);
          setError(reason instanceof Error ? reason.message : "检索未完成");
          if (reason instanceof ApiError && reason.status === 401) onError(reason);
        })
        .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    }, 250);
    return () => { window.clearTimeout(timer); controller.abort(); };
  }, [query, mode, onError]);

  async function ask() {
    askController.current?.abort();
    const controller = new AbortController();
    askController.current = controller;
    setAsking(true);
    setAnswer(null);
    try {
      const result = await api.ask(query.trim(), controller.signal);
      if (!controller.signal.aborted) setAnswer({ text: result.answer, citations: result.citations });
    } catch (reason) {
      if (!controller.signal.aborted) onError(reason);
    } finally {
      if (!controller.signal.aborted) setAsking(false);
    }
  }

  const valid = query.trim().length > 0 && Array.from(query.trim()).length <= 200;
  return <section className="knowledge-panel" aria-label="本班知识检索">
    <div className="knowledge-search">
      <label htmlFor="knowledge-query">检索本班知识片段</label>
      <div className="knowledge-query-row">
        <input id="knowledge-query" value={query} onChange={(event) => setQuery(event.target.value)} maxLength={200} placeholder="输入原词或用自然语言提问" />
        <button className="primary" onClick={ask} disabled={!valid || asking}>{asking ? "回答中…" : "依据资料回答"}</button>
      </div>
      <div className="mode-switch" role="group" aria-label="检索模式">
        {(["keyword", "vector", "hybrid"] as SearchMode[]).map((value) => <button key={value} className={mode === value ? "selected" : ""} onClick={() => setMode(value)} aria-pressed={mode === value}>{value === "keyword" ? "关键字" : value === "vector" ? "语义" : "混合"}</button>)}
      </div>
      <p className="search-explainer">关键字匹配原文；语义查找相近表述；混合融合两路排序。所有结果仅来自当前班级。</p>
    </div>
    {indexState === "building" && <p className="index-notice">本班材料正在建立索引，结果可能暂时不完整。</p>}
    {indexState === "degraded" && <p className="index-notice warning">部分材料索引失败；可联系教师重建。</p>}
    {answer && <article className="answer-card"><p className="eyebrow">依据资料回答</p><p>{answer.text}</p>{answer.citations.length > 0 && <div className="answer-citations">{answer.citations.map((hit, index) => <button key={hit.chunk_id} onClick={() => onOpen(hit)}>[{index + 1}] {hit.title} · 切片 {hit.chunk_index}</button>)}</div>}</article>}
    {error && <p className="form-error" role="alert">{error}</p>}
    {loading ? <p className="knowledge-state">正在检索本班资料…</p> : valid && !error && hits.length === 0 ? <p className="knowledge-state">资料中未找到相关内容</p> : null}
    <div className="knowledge-results" aria-live="polite">
      {hits.map((hit) => <article className="knowledge-hit" key={hit.chunk_id}>
        <div className="hit-meta"><span>#{hit.rank} · 切片 {hit.chunk_index}</span><span>{hit.offset_basis === "original" ? "原文" : "预处理文本"}字符 {hit.start_offset}–{hit.end_offset}</span></div>
        <h2>{hit.title}</h2><p className="hit-file">{hit.original_name}</p>
        <blockquote>{hit.excerpt}</blockquote>
        <button className="secondary" onClick={() => onOpen(hit)}>打开来源</button>
      </article>)}
    </div>
  </section>;
}

function DetailDrawer({ item, citation, onClose, onDownload, onReindex }: { item: Material; citation: KnowledgeHit | null; onClose: () => void; onDownload: () => void; onReindex?: (strategy: ChunkStrategy) => Promise<void> }) {
  const isMarkdown = item.original_name.toLowerCase().endsWith(".md");
  const [strategy, setStrategy] = useState<ChunkStrategy>({ mode: "auto" });
  const [reindexBusy, setReindexBusy] = useState(false);
  const [reindexError, setReindexError] = useState("");
  const content = item.content || "";
  const characters = Array.from(content);
  const matchesCitation = citation?.offset_basis === "original" &&
    characters.slice(citation.start_offset, citation.end_offset).join("") === citation.excerpt;

  async function reindex() {
    if (!onReindex) return;
    setReindexBusy(true);
    setReindexError("");
    try { await onReindex(strategy.mode === "custom" ? { max_length: 800, overlap_percent: 10, separator: "newline", ...strategy } : strategy); }
    catch (reason) { setReindexError(reason instanceof Error ? reason.message : "重建失败"); }
    finally { setReindexBusy(false); }
  }
  return (
    <div className="overlay" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <aside className="detail-drawer" role="dialog" aria-modal="true" aria-labelledby="detail-title">
        <header>
          <div><p>{item.class_name} · {isMarkdown ? "Markdown" : "纯文本"}</p><h2 id="detail-title">{item.title}</h2></div>
          <button className="close-button" onClick={onClose} aria-label="关闭详情">×</button>
        </header>
        <div className="detail-meta">
          <span>{formatDate(item.created_at)}</span><span>{formatBytes(item.size_bytes)}</span><span>{item.original_name}</span>
        </div>
        <article className={`document-body ${isMarkdown ? "markdown-body" : "plain-body"}`}>
          {citation && <p className="citation-note">来源：切片 {citation.chunk_index} · {citation.offset_basis === "original" ? "原文字符区间" : "预处理文本字符区间"} {citation.start_offset}–{citation.end_offset}</p>}
          {citation?.offset_basis === "processed" && <blockquote className="processed-excerpt">{citation.excerpt}</blockquote>}
          {matchesCitation ? <pre>{characters.slice(0, citation!.start_offset).join("")}<mark>{citation!.excerpt}</mark>{characters.slice(citation!.end_offset).join("")}</pre> : isMarkdown ? <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown> : <pre>{content}</pre>}
        </article>
        <footer className="detail-footer"><button className="primary" onClick={onDownload}>下载原文件</button>{onReindex && <div className="reindex-controls"><StrategyFields value={strategy} onChange={setStrategy} /><button className="secondary" onClick={reindex} disabled={reindexBusy}>{reindexBusy ? "排队中…" : "按此策略重建索引"}</button>{reindexError && <span role="alert">{reindexError}</span>}</div>}</footer>
      </aside>
    </div>
  );
}

function StrategyFields({ value, onChange }: { value: ChunkStrategy; onChange: (value: ChunkStrategy) => void }) {
  return <div className="strategy-fields">
    <label>切分策略
      <select value={value.mode} onChange={(event) => onChange({ mode: event.target.value as ChunkStrategy["mode"] })}>
        <option value="auto">自动窗口 · 800 字 / 重叠 80 字</option>
        <option value="custom">自定义分隔符</option>
        <option value="hierarchy">Markdown 标题分章</option>
      </select>
    </label>
    {value.mode === "custom" && <div className="strategy-custom">
      <label>最大字数<input type="number" min={100} max={2000} value={value.max_length ?? 800} onChange={(event) => onChange({ ...value, max_length: Number(event.target.value), separator: value.separator ?? "newline" })} /></label>
      <label>重叠百分比<input type="number" min={0} max={50} value={value.overlap_percent ?? 10} onChange={(event) => onChange({ ...value, overlap_percent: Number(event.target.value), separator: value.separator ?? "newline", max_length: value.max_length ?? 800 })} /></label>
      <label>分隔符<select value={value.separator ?? "newline"} onChange={(event) => onChange({ ...value, separator: event.target.value as ChunkStrategy["separator"], max_length: value.max_length ?? 800 })}><option value="newline">换行</option><option value="blank-line">空行</option><option value="period">句号</option></select></label>
      <label className="check-option"><input type="checkbox" checked={value.remove_urls ?? false} onChange={(event) => onChange({ ...value, remove_urls: event.target.checked, separator: value.separator ?? "newline", max_length: value.max_length ?? 800 })} />移除 URL</label>
      <label className="check-option"><input type="checkbox" checked={value.remove_emails ?? false} onChange={(event) => onChange({ ...value, remove_emails: event.target.checked, separator: value.separator ?? "newline", max_length: value.max_length ?? 800 })} />移除邮箱</label>
      <label className="check-option"><input type="checkbox" checked={value.collapse_whitespace ?? false} onChange={(event) => onChange({ ...value, collapse_whitespace: event.target.checked, separator: value.separator ?? "newline", max_length: value.max_length ?? 800 })} />合并连续空白</label>
    </div>}
  </div>;
}

function UploadDialog({ onClose, onUploaded, onError }: { onClose: () => void; onUploaded: () => void; onError: (reason: unknown) => void }) {
  const [file, setFile] = useState<File | null>(null);
  const [title, setTitle] = useState("");
  const [progress, setProgress] = useState(0);
  const [busy, setBusy] = useState(false);
  const [strategy, setStrategy] = useState<ChunkStrategy>({ mode: "auto" });

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!file) return;
    setBusy(true);
    try {
      await uploadMaterial(file, title, strategy.mode === "custom" ? { max_length: 800, overlap_percent: 10, separator: "newline", ...strategy } : strategy, setProgress);
      onUploaded();
    } catch (reason) {
      onError(reason);
      if (reason instanceof ApiError && reason.status === 401) onClose();
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="overlay centered" role="presentation" onMouseDown={(event) => !busy && event.target === event.currentTarget && onClose()}>
      <form className="upload-dialog" role="dialog" aria-modal="true" aria-labelledby="upload-title" onSubmit={submit}>
        <header><div><p>教师操作</p><h2 id="upload-title">上传并入库</h2></div><button type="button" className="close-button" onClick={onClose} disabled={busy}>×</button></header>
        <label className={`drop-field ${file ? "has-file" : ""}`}>
          <input type="file" accept=".txt,.md,text/plain,text/markdown" onChange={(event) => setFile(event.target.files?.[0] || null)} required />
          <span className="upload-glyph">＋</span>
          <strong>{file ? file.name : "选择 .txt 或 .md 文件"}</strong>
          <small>{file ? formatBytes(file.size) : "文件正文会写入本班知识库"}</small>
        </label>
        <label>材料标题（可选）<input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="默认使用文件名" maxLength={255} /></label>
        <div className="upload-strategy"><StrategyFields value={strategy} onChange={setStrategy} /></div>
        {busy && <div className="progress-block"><div><span>正在上传并入库</span><strong>{progress}%</strong></div><progress max="100" value={progress} /></div>}
        <div className="dialog-actions"><button type="button" className="secondary" onClick={onClose} disabled={busy}>取消</button><button className="primary" disabled={!file || busy}>{busy ? "处理中…" : "上传材料"}</button></div>
      </form>
    </div>
  );
}

function CommandPalette({ commands, onClose }: { commands: { label: string; hint: string; run: () => void }[]; onClose: () => void }) {
  const [query, setQuery] = useState("");
  const visible = commands.filter((item) => item.label.includes(query));
  return (
    <div className="overlay command-overlay" role="presentation" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
      <section className="command-palette" role="dialog" aria-modal="true" aria-label="快捷命令">
        <div className="command-search"><span>⌘</span><input autoFocus value={query} onChange={(event) => setQuery(event.target.value)} placeholder="输入命令" /></div>
        <div className="command-list">
          {visible.map((command) => <button key={command.label} onClick={() => { onClose(); command.run(); }}><span>{command.label}</span><kbd>{command.hint}</kbd></button>)}
          {visible.length === 0 && <p>没有匹配的命令</p>}
        </div>
        <footer><span>↑↓ 选择</span><span>Enter 执行</span><span>Esc 关闭</span></footer>
      </section>
    </div>
  );
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

function formatDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", { month: "short", day: "numeric", year: "numeric" }).format(date);
}

export default App;

import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { ApiError, Material, User, api, downloadMaterial, uploadMaterial } from "./api";

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

  async function openMaterial(item: Material) {
    try {
      const result = await api.material(item.id);
      setSelected(result.material);
    } catch (reason) {
      handleError(reason);
    }
  }

  async function logout() {
    try {
      await api.logout();
    } finally {
      onUnauthorized();
    }
  }

  const commands = useMemo(() => [
    { label: "搜索本班材料", hint: "/", run: () => searchRef.current?.focus() },
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
          <button className="nav-item active"><span>册</span>材料库</button>
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
            <p className="location">{user.class_name} / 材料库</p>
            <h1>教研材料</h1>
            <p className="header-note">共 {materials.length} 份可见材料 · 权限来自服务端会话</p>
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

        <section className="toolbar" aria-label="材料筛选与视图">
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
        </section>
      </main>

      {selected && <DetailDrawer item={selected} onClose={() => setSelected(null)} onDownload={() => downloadMaterial(selected).then(() => toast("下载已开始", "success")).catch(handleError)} />}
      {uploadOpen && user.role === "teacher" && <UploadDialog onClose={() => setUploadOpen(false)} onUploaded={() => { setUploadOpen(false); loadMaterials(query); toast("材料已入库，本班现在可以查看", "success"); }} onError={handleError} />}
      {commandsOpen && <CommandPalette commands={commands} onClose={() => setCommandsOpen(false)} />}
      <div className="toast-region" aria-live="polite">
        {toasts.map((item) => <div className={`toast ${item.tone}`} key={item.id}>{item.message}</div>)}
      </div>
    </div>
  );
}

function DetailDrawer({ item, onClose, onDownload }: { item: Material; onClose: () => void; onDownload: () => void }) {
  const isMarkdown = item.original_name.toLowerCase().endsWith(".md");
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
          {isMarkdown ? <ReactMarkdown remarkPlugins={[remarkGfm]}>{item.content || ""}</ReactMarkdown> : <pre>{item.content}</pre>}
        </article>
        <footer><button className="primary" onClick={onDownload}>下载原文件</button></footer>
      </aside>
    </div>
  );
}

function UploadDialog({ onClose, onUploaded, onError }: { onClose: () => void; onUploaded: () => void; onError: (reason: unknown) => void }) {
  const [file, setFile] = useState<File | null>(null);
  const [title, setTitle] = useState("");
  const [progress, setProgress] = useState(0);
  const [busy, setBusy] = useState(false);

  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!file) return;
    setBusy(true);
    try {
      await uploadMaterial(file, title, setProgress);
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


import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { api, applyTutorEvent, Conversation, KnowledgeHit, sendTutorMessage, TutorAgent, TutorMessage } from "./api";

const preferredApps = ["数学解题助手", "语文阅读助手", "英语学习助手"];
const appOrder = (name: string) => preferredApps.includes(name) ? preferredApps.indexOf(name) : preferredApps.length;

export default function TutorChat({ onOpen, onError }: { onOpen: (hit: KnowledgeHit) => void; onError: (error: unknown) => void }) {
  const [agents, setAgents] = useState<TutorAgent[]>([]), [conversations, setConversations] = useState<Conversation[]>([]);
  const [agentID, setAgentID] = useState(0), [current, setCurrent] = useState<Conversation | null>(null), [messages, setMessages] = useState<TutorMessage[]>([]);
  const [question, setQuestion] = useState(""), [busy, setBusy] = useState(false), [loading, setLoading] = useState(true), [error, setError] = useState("");
  const controller = useRef<AbortController | null>(null), bottom = useRef<HTMLDivElement>(null);
  const refresh = useCallback(async () => {
    const [a, c] = await Promise.all([api.agents(), api.conversations()]); setAgents(a.agents); setConversations(c.conversations);
    setAgentID((old) => old || a.agents.find((item) => item.enabled)?.id || 0);
  }, []);
  useEffect(() => { refresh().catch((e) => { setError(e.message); onError(e); }).finally(() => setLoading(false)); return () => controller.current?.abort(); }, [refresh, onError]);
  useEffect(() => { bottom.current?.scrollIntoView({ block: "nearest" }); }, [messages]);
  async function select(id: number) {
    if (busy) return; setLoading(true); setError("");
    try { const data = await api.conversation(id); setCurrent(data.conversation); setMessages(data.messages); } catch (e) { onError(e); setError(e instanceof Error ? e.message : "会话读取失败"); } finally { setLoading(false); }
  }
  async function create(selectedID = agentID) {
    if (!selectedID || busy) return; setAgentID(selectedID); setBusy(true); setError("");
    try { const c = await api.createConversation(selectedID); setCurrent(c); setMessages([]); await refresh(); } catch (e) { onError(e); setError(e instanceof Error ? e.message : "新建失败"); } finally { setBusy(false); }
  }
  async function submit(event: FormEvent) {
    event.preventDefault(); const text = question.trim(); if (!current || busy || Array.from(text).length < 2 || Array.from(text).length > 500) return;
    setBusy(true); setError(""); setQuestion(""); const c = new AbortController(); controller.current = c;
    const now = new Date().toISOString();
    setMessages((old) => [...old, { id: -1, role: "user", content: text, citations: [], skill_name: "", status: "complete", created_at: now }, { id: -2, role: "assistant", content: "", citations: [], skill_name: "", status: "incomplete", created_at: now }]);
    try {
      await sendTutorMessage(current.id, text, ({ event: kind, data }) => {
        if (kind === "error") throw new Error(data.error || "回答未完成");
        setMessages((old) => old.map((m, i) => i !== old.length - 1 ? m : applyTutorEvent(m, {event: kind, data})));
      }, c.signal);
      const saved = await api.conversation(current.id); setMessages(saved.messages); setCurrent(saved.conversation);
    } catch (e) {
      if (e instanceof DOMException && e.name === "AbortError") setError("已停止生成，已有文字会保留为未完成回答。");
      else { setError(e instanceof Error ? e.message : "回答未完成"); onError(e); }
    } finally { controller.current = null; setBusy(false); void refresh().catch(onError); }
  }
  const enabledAgents = agents.filter(a=>a.enabled).sort((a,b)=>appOrder(a.name)-appOrder(b.name));
  return <section className="tutor-layout">
    <aside className="conversation-sidebar">
      <p className="eyebrow">本班解题助手</p>
      <label>新会话使用的智能体<select aria-label="选择智能体" value={agentID} disabled={busy || loading} onChange={(e) => setAgentID(Number(e.target.value))}><option value={0}>请选择助手</option>{agents.filter((a) => a.enabled).map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}</select></label>
      <button className="primary wide" onClick={() => void create()} disabled={!agentID || busy || loading}>新建会话</button>
      {!loading && !agents.some((a) => a.enabled) && <p className="muted">本班暂无启用的助手，请教师先完成配置。</p>}
      <h2>我的会话</h2>
      <div className="conversation-list">{conversations.map((c) => <button key={c.id} disabled={busy} className={current?.id === c.id ? "selected" : ""} onClick={() => select(c.id)}>{c.title}<small>{new Date(c.updated_at).toLocaleString("zh-CN")}</small></button>)}</div>
      {!loading && !conversations.length && <p className="muted">还没有会话，选择助手后开始提问。</p>}
    </aside>
    <div className="conversation-main">
      <div className="conversation-heading"><h2>{current?.title || "开始一次学习对话"}</h2><p className="muted">{current ? `当前助手：${agents.find((a) => a.id === current.agent_id)?.name || `助手 #${current.agent_id}`} · 更换助手请新建会话` : "答案逐段出现，引用资料先到达。"}</p></div>
      {!current && !loading && <section className="application-catalog" aria-label="学习应用"><h3>选择一个学习应用</h3><p className="muted">选择助手即可开始；教师可在助手配置中调整技能与资料。</p><div className="application-grid">{enabledAgents.map(a=><button type="button" className="application-card" key={a.id} disabled={busy} aria-label={`使用${a.name}`} onClick={()=>void create(a.id)}><strong>{a.name}</strong><span>{a.prompt.slice(0,100)||"按你的问题提供学习帮助"}</span><small>开始对话 →</small></button>)}</div></section>}
      <div className="message-list" aria-busy={busy || loading} aria-label="会话消息">
        {loading && <p className="muted">正在读取会话…</p>}
        {!loading && current && !messages.length && <p className="muted">说说你已经做到哪一步，以及卡住的地方。</p>}
        {messages.map((m, i) => <article className={`tutor-message ${m.role}`} key={`${m.role}-${i}`}><p className="message-role">{m.role === "user" ? "我的提问" : "助手"}{m.skill_name && ` · ${m.skill_name}`}{m.role === "assistant" && m.status === "incomplete" && !busy && " · 未完成"}</p>
          {m.citations.length > 0 && <div className="tutor-citations" aria-label="引用资料">{m.citations.map((hit, n) => <button className="citation-link" key={`${hit.chunk_id}-${n}`} onClick={() => onOpen(hit)}>[{n + 1}] {hit.title} · 切片 {hit.chunk_index}</button>)}</div>}
          {!!m.tool_calls?.length && <div className="tool-call-list" aria-label="工具调用">{m.tool_calls.map(call => <div className={`tool-call ${call.status}`} key={call.call_id}><strong>{call.ref}</strong><span> · 第 {call.round} 轮 · {({ok: "成功", error: "出错", denied: "被拒"})[call.status]}</span><p>{call.text}</p></div>)}</div>}
          {m.role === "assistant" ? <div className="markdown-body"><ReactMarkdown remarkPlugins={[remarkGfm]}>{m.content || (busy ? "正在思考…" : "本轮未生成正文")}</ReactMarkdown></div> : <p className="question-text">{m.content}</p>}
        </article>)}<div ref={bottom} />
      </div>
      {error && <p className="form-error" role="alert">{error}</p>}
      <form className="tutor-composer" onSubmit={submit}><label>你的问题<textarea aria-label="你的问题" placeholder="例如：去括号时这一步哪里不对？" value={question} disabled={!current || busy} onChange={(e) => setQuestion(e.target.value)} /></label><div><small className="muted">{Array.from(question.trim()).length}/500 字</small>{busy ? <button className="secondary" type="button" onClick={() => controller.current?.abort()}>停止生成</button> : <button className="primary" disabled={!current || loading || Array.from(question.trim()).length < 2 || Array.from(question.trim()).length > 500}>发送问题</button>}</div></form>
    </div>
  </section>;
}

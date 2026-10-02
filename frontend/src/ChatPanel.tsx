import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { ApiError, KnowledgeHit, api } from "./api";
import { ChatRound, chatHistory } from "./chat-history";

export default function ChatPanel({ active, onOpen, onError }: {
  active: boolean;
  onOpen: (hit: KnowledgeHit) => void;
  onError: (reason: unknown) => void;
}) {
  const [draft, setDraft] = useState("");
  const [rounds, setRounds] = useState<ChatRound[]>([]);
  const [busy, setBusy] = useState(false);
  const request = useRef<{ controller: AbortController; id: number } | null>(null);
  const nextID = useRef(0);
  const messages = useRef<HTMLDivElement>(null);
  const composer = useRef<HTMLTextAreaElement>(null);
  const length = Array.from(draft.trim()).length;
  const valid = length > 0 && length <= 200;

  const stop = useCallback(() => {
    const current = request.current;
    if (!current) return;
    current.controller.abort();
    request.current = null;
    setBusy(false);
    setRounds((items) => items.map((round) => round.id === current.id ? { ...round, status: "stopped" } : round));
  }, []);

  useEffect(() => {
    if (!active) stop();
  }, [active, stop]);

  useEffect(() => () => {
    request.current?.controller.abort();
    request.current = null;
  }, []);

  useEffect(() => {
    if (active && messages.current) messages.current.scrollTop = messages.current.scrollHeight;
  }, [rounds, active]);

  async function send(retry?: ChatRound) {
    if (!active || request.current || (!retry && !valid)) return;
    const question = retry?.question ?? draft.trim();
    const id = retry?.id ?? ++nextID.current;
    const controller = new AbortController();
    request.current = { controller, id };
    const history = chatHistory(retry ? rounds.filter((round) => round.id < id) : rounds);
    setBusy(true);
    const pending: ChatRound = { id, question, status: "pending" };
    setRounds((items) => retry ? items.map((round) => round.id === id ? pending : round) : [...items, pending]);
    if (!retry) setDraft("");

    try {
      const result = await api.ask(question, history, controller.signal);
      if (!controller.signal.aborted) {
        setRounds((items) => items.map((round) => round.id === id ? { ...round, status: "answered", result } : round));
      }
    } catch (reason) {
      if (!controller.signal.aborted) {
        setRounds((items) => items.map((round) => round.id === id ? {
          ...round, status: "failed", error: reason instanceof Error ? reason.message : "连接失败，请稍后重试"
        } : round));
        if (reason instanceof ApiError && reason.status === 401) onError(reason);
      }
    } finally {
      if (request.current?.controller === controller) {
        request.current = null;
        setBusy(false);
        composer.current?.focus();
      }
    }
  }

  function clear() {
    stop();
    setRounds([]);
    setDraft("");
    composer.current?.focus();
  }

  function submit(event: FormEvent) {
    event.preventDefault();
    void send();
  }

  return <section className="chat-panel" aria-label="本班知识问答" hidden={!active}>
    <header className="chat-header">
      <p>回答依据本班资料，点击来源核对原文。</p>
      <button className="secondary" onClick={clear} disabled={rounds.length === 0 && !draft}>清空对话</button>
    </header>
    <div className="chat-messages" ref={messages} role="log" aria-label="问答消息" aria-live="polite" aria-relevant="additions text">
      {rounds.length === 0 && <div className="chat-welcome">
        <div className="chat-avatar" aria-hidden="true">问</div>
        <h2>想了解本班资料中的什么？</h2>
        <p>可以提问、追问，也可以打开每轮回答的材料来源。</p>
        <button className="chat-suggestion" onClick={() => { setDraft("本班资料中有哪些教研重点？"); composer.current?.focus(); }}>本班资料中有哪些教研重点？</button>
      </div>}
      {rounds.map((round, index) => <div className="chat-round" key={round.id}>
        <article className="chat-message user-message" aria-label="我的问题">
          <span className="message-author">你</span>
          <div className="message-bubble"><p>{round.question}</p></div>
        </article>
        <article className="chat-message assistant-message" aria-label="助手回答" aria-busy={round.status === "pending"}>
          <span className="message-author">资料助手</span>
          <div className="message-bubble">
            {round.status === "pending" && <p className="chat-pending"><span className="status-dot" aria-hidden="true" />正在查找资料并回答…</p>}
            {round.status === "answered" && round.result && <>
              <div className="chat-answer markdown-body"><ReactMarkdown remarkPlugins={[remarkGfm]} skipHtml>{round.result.answer}</ReactMarkdown></div>
              {round.result.index_state === "building" && <p className="index-notice">本班材料正在建立索引，回答依据可能暂时不完整。</p>}
              {round.result.index_state === "degraded" && <p className="index-notice warning">部分材料索引失败，可联系教师重建。</p>}
              {round.result.citations.length > 0 && <div className="chat-sources">
                <p>本轮来源</p>
                <div className="answer-citations">{round.result.citations.map((hit, citationIndex) => <button key={hit.chunk_id} onClick={() => onOpen(hit)} title={hit.excerpt}>[{citationIndex + 1}] {hit.title} · 切片 {hit.chunk_index}</button>)}</div>
              </div>}
            </>}
            {round.status === "failed" && <p className="chat-error" role="alert">{round.error}。可以重试此问题。</p>}
            {round.status === "stopped" && <p className="chat-stopped">已停止回答</p>}
            {(round.status === "failed" || round.status === "stopped") && index === rounds.length - 1 && <button className="secondary chat-retry" disabled={busy} onClick={() => void send(round)}>重试此问题</button>}
          </div>
        </article>
      </div>)}
    </div>
    <form className="chat-composer" onSubmit={submit}>
      <label htmlFor="chat-question">向本班资料提问</label>
      <textarea id="chat-question" ref={composer} value={draft} rows={3} placeholder="输入问题，或继续追问…" aria-describedby="chat-input-hint" aria-invalid={length > 200} onChange={(event) => setDraft(event.target.value)} onKeyDown={(event) => {
        if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing && event.nativeEvent.keyCode !== 229) {
          event.preventDefault();
          void send();
        }
      }} />
      <div className="chat-composer-footer">
        <p id="chat-input-hint" className={length > 200 ? "chat-input-error" : ""}>{length > 200 ? "问题最多 200 字，请缩短后发送" : "Enter 发送 · Shift+Enter 换行"}<span>{length}/200</span></p>
        {busy ? <button key="stop" type="button" className="secondary" onClick={stop}>停止回答</button> : <button key="send" type="submit" className="primary" disabled={!valid}>发送问题</button>}
      </div>
    </form>
  </section>;
}

import { describe, expect, it, vi } from "vitest";

vi.stubGlobal("sessionStorage", { getItem: () => null, setItem: () => undefined, removeItem: () => undefined });
const { readEventStream, sendTutorMessage, api } = await import("./api");
function stream(text: string) { const bytes = new TextEncoder().encode(text); return new ReadableStream<Uint8Array>({ start(c) { for (const byte of bytes) c.enqueue(new Uint8Array([byte])); c.close(); } }); }
describe("Tutor SSE", () => {
  it("decodes split UTF-8, CRLF and event boundaries without duplicate text", async () => {
    const events: string[] = [], text: string[] = [];
    await readEventStream(stream('event: meta\r\ndata: {"citations":[]}\r\n\r\nevent: delta\ndata: {"text":"中文"}\n\nevent: delta\ndata: {"text":"增量"}\n\nevent: done\ndata: {"message_id":1}\n\n'), (e) => { events.push(e.event); if (e.data.text) text.push(e.data.text); });
    expect(events).toEqual(["meta", "delta", "delta", "done"]); expect(text.join("")).toBe("中文增量");
  });
  it("reports errors and rejects truncated streams", async () => {
    const events: string[] = [];
    await readEventStream(stream('event: error\ndata: {"error":"failed"}\n\n'), (e) => events.push(e.event)); expect(events).toEqual(["error"]);
    await expect(readEventStream(stream('event: delta\ndata: {"text":"partial"}\n\n'), () => undefined)).rejects.toThrow("未收到结束事件");
  });
  it("honors cancellation", async () => { const c = new AbortController(); c.abort(); await expect(readEventStream(stream(""), () => undefined, c.signal)).rejects.toMatchObject({ name: "AbortError" }); });
  it("sends Bearer and omits cookies, including HTTP failures", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValueOnce(new Response(JSON.stringify({ user: {}, token: "test-token" }), { status: 200, headers: { "Content-Type": "application/json" } })).mockResolvedValueOnce(new Response(JSON.stringify({ error: "需要登录" }), { status: 401, headers: { "Content-Type": "application/json" } })));
    await api.login("test", "test"); await expect(sendTutorMessage(1, "问题", () => undefined, new AbortController().signal)).rejects.toMatchObject({ status: 401 });
    const call = vi.mocked(fetch).mock.calls[1]; expect(call[1]?.credentials).toBe("omit"); expect((call[1]?.headers as Record<string, string>).Authorization).toBe("Bearer test-token");
  });
});

describe("MCP stream metadata", () => {
 it("merges repeated meta by call id without losing text, citations or the previous round", async () => {
  const {applyTutorEvent} = await import("./api");
  let message: import("./api").TutorMessage = {id: -2, role: "assistant", content: "已有文字", citations: [], skill_name: "技能", status: "incomplete", created_at: ""};
  const call: import("./api").ToolCall = {ref: "formula_hint@campus-tools", status: "ok", text: "提示", round: 1, call_id: "a"};
  message = applyTutorEvent(message, {event: "meta", data: {tool_calls: [call]}});
  message = applyTutorEvent(message, {event: "meta", data: {tool_calls: [call, {...call, call_id: "b", round: 2, status: "denied"}]}});
  message = applyTutorEvent(message, {event: "meta", data: {}});
  expect(message.tool_calls).toHaveLength(2);expect(message.content).toBe("已有文字");expect(message.skill_name).toBe("技能");
  message = applyTutorEvent(message, {event: "delta", data: {text: "后续"}});expect(message.content).toBe("已有文字后续");
  message = applyTutorEvent(message, {event: "done", data: {message_id: 12}});expect(message.status).toBe("complete");expect(message.id).toBe(12);
 });
 it("decodes heartbeats and multiple per-round tool events over single-byte reads", async () => {
  const events: string[] = [];
  await readEventStream(stream(': tools pending\n\nevent: meta\ndata: {"skill":null,"citations":[]}\n\nevent: meta\ndata: {"tool_calls":[{"ref":"formula_hint@campus-tools","status":"ok","text":"提示","round":1,"call_id":"c"}]}\n\nevent: delta\ndata: {"text":"正文"}\n\nevent: done\ndata: {"message_id":1}\n\n'), e => events.push(e.event));
  expect(events).toEqual(["meta", "meta", "delta", "done"]);
 });
});

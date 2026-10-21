export type Role = "teacher" | "student";

export interface User {
  id: number;
  username: string;
  role: Role;
  class_id: number;
  class_name: string;
}

export interface Material {
  id: number;
  class_id: number;
  class_name: string;
  title: string;
  original_name: string;
  media_type: string;
  size_bytes: number;
  created_at: string;
  content?: string;
}

export type SearchMode = "keyword" | "vector" | "hybrid";
export type IndexState = "ready" | "building" | "degraded";

export interface KnowledgeHit {
  chunk_id: number;
  material_id: number;
  title: string;
  original_name: string;
  chunk_index: number;
  excerpt: string;
  start_offset: number;
  end_offset: number;
  offset_basis: "original" | "processed";
  keyword_score?: number;
  vector_score?: number;
  rank: number;
}

export interface KnowledgeSearchResult {
  hits: KnowledgeHit[];
  message?: string;
  index_state: IndexState;
}

export interface AskResult {
  answer: string;
  citations: KnowledgeHit[];
  index_state: IndexState;
}

export interface HistoryTurn {
  role: "user" | "assistant";
  content: string;
}

export interface ChunkStrategy {
  mode: "auto" | "custom" | "hierarchy";
  max_length?: number;
  overlap_percent?: number;
  separator?: "newline" | "blank-line" | "period";
  remove_urls?: boolean;
  remove_emails?: boolean;
  collapse_whitespace?: boolean;
}

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

// Start the token-only flow with a fresh login after upgrading from the cookie-compatible build.
sessionStorage.removeItem("campus-access-token");
const tokenKey = "campus-access-token-v2";
let accessToken = sessionStorage.getItem(tokenKey) || "";

export function hasAccessToken() {
  return accessToken !== "";
}

export function clearAccessToken() {
  accessToken = "";
  sessionStorage.removeItem(tokenKey);
}

function saveAccessToken(token: string) {
  accessToken = token;
  sessionStorage.setItem(tokenKey, token);
}

async function request<T>(path: string, options: RequestInit = {}, sendToken = true): Promise<T> {
  const headers = new Headers(options.headers);
  if (options.body && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  if (sendToken && accessToken && !headers.has("Authorization")) headers.set("Authorization", `Bearer ${accessToken}`);
  const response = await fetch(path, {
    ...options,
    credentials: "omit",
    headers
  });
  if (!response.ok) {
    let message = "请求未完成";
    try {
      const body = (await response.json()) as { error?: string };
      message = body.error || message;
    } catch {
      // Keep the safe generic message for non-JSON failures.
    }
    throw new ApiError(response.status, message);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export const api = {
 mcpServers: () => request<{servers: MCPServer[]}>("/api/mcp/servers"),
 saveMCPServer: (input: {name: string; base_url: string; token?: string}, id?: number) => request<MCPServer>(id ? `/api/mcp/servers/${id}` : "/api/mcp/servers", {method: id ? "PUT" : "POST", body: JSON.stringify(input)}),
 deleteMCPServer: (id: number) => request<void>(`/api/mcp/servers/${id}`, {method: "DELETE"}),
 connectMCP: (id: number) => request<{server: MCPServer; tools: MCPTool[]}>(`/api/mcp/servers/${id}/connect`, {method: "POST"}),
 disconnectMCP: (id: number) => request<MCPServer>(`/api/mcp/servers/${id}/disconnect`, {method: "POST"}),
 mcpTools: (id: number) => request<{tools: MCPTool[]}>(`/api/mcp/servers/${id}/tools`),
 mcpChanges: (id: number, before?: number) => request<{changes: MCPChange[]; next_before: number}>(`/api/mcp/servers/${id}/changes?limit=20${before ? `&before=${before}` : ""}`),
 mcpAudits: (before = "", status = "", serverID = "") => request<{audits: MCPAudit[]; next_before: string}>(`/api/mcp/audits?${new URLSearchParams({limit: "20", ...(before ? {before} : {}), ...(status ? {status} : {}), ...(serverID ? {server_id: serverID} : {})})}`),
  promptTemplates: () => request<{templates: PromptTemplate[]}>("/api/prompt-templates"),
  savePromptTemplate: (input: {name: string; prompt: string}, id?: number) => request<PromptTemplate>(id ? `/api/prompt-templates/${id}` : "/api/prompt-templates", {method: id ? "PUT" : "POST", body: JSON.stringify(input)}),
  deletePromptTemplate: (id: number) => request<void>(`/api/prompt-templates/${id}`, {method:"DELETE"}),
  agents: () => request<{ agents: TutorAgent[] }>("/api/agents"),
  saveAgent: (agent: Omit<TutorAgent, "id" | "class_id">, id?: number) => request<TutorAgent>(id ? `/api/agents/${id}` : "/api/agents", { method: id ? "PUT" : "POST", body: JSON.stringify(agent) }),
  skills: () => request<{ skills: TutorSkill[] }>("/api/skills"),
  previewSkill: (source: string) => request<SkillPreview>("/api/skills/preview", {method: "POST", body: JSON.stringify({source})}),
  saveSkill: (source: string, id?: number, routing?: SkillRouting) => request<TutorSkill>(id ? `/api/skills/${id}` : "/api/skills", { method: id ? "PUT" : "POST", body: JSON.stringify({ source, ...(routing ? {routing} : {}) }) }),
  deleteSkill: (id: number) => request<void>(`/api/skills/${id}`, { method: "DELETE" }),
  conversations: () => request<{ conversations: Conversation[] }>("/api/conversations"),
  createConversation: (agent_id: number) => request<Conversation>("/api/conversations", { method: "POST", body: JSON.stringify({ agent_id }) }),
  conversation: (id: number) => request<{ conversation: Conversation; messages: TutorMessage[] }>(`/api/conversations/${id}`),
  me: () => request<{ user: User }>("/api/me"),
  login: async (username: string, password: string) => {
    const result = await request<{ user: User; token: string }>("/api/login", {
      method: "POST",
      body: JSON.stringify({ username, password })
    }, false);
    saveAccessToken(result.token);
    return result;
  },
  useToken: async (value: string) => {
    const token = value.trim().replace(/^Bearer\s+/i, "");
    const result = await request<{ user: User }>("/api/me", {
      headers: { Authorization: `Bearer ${token}` }
    });
    saveAccessToken(token);
    return result;
  },
  logout: async () => {
    try {
      await request<void>("/api/logout", { method: "POST" });
    } finally {
      clearAccessToken();
    }
  },
  materials: (query: string) =>
    request<{ materials: Material[] }>(`/api/materials?q=${encodeURIComponent(query)}`),
  material: (id: number) => request<{ material: Material }>(`/api/materials/${id}`),
  deleteMaterial: (id: number) => request<void>(`/api/materials/${id}`, { method: "DELETE" }),
  knowledgeSearch: (query: string, mode: SearchMode, signal?: AbortSignal) => {
    const params = new URLSearchParams({ q: query, mode });
    return request<KnowledgeSearchResult>(`/api/knowledge/search?${params}`, { signal });
  },
  ask: (question: string, history: HistoryTurn[] = [], signal?: AbortSignal) => request<AskResult>("/api/ask", {
    method: "POST", body: JSON.stringify({ question, history }), signal
  }),
  reindex: (id: number, strategy: ChunkStrategy) => request<{ message: string }>(`/api/materials/${id}/reindex`, {
    method: "POST", body: JSON.stringify({ strategy })
  })
};

export interface TutorAgent { id: number; class_id: number; name: string; prompt: string; enabled: boolean; skill_ids: number[]; material_ids: number[] }
export interface TutorSkill { id: number; name: string; trigger: string; subject: string; description: string; source: string; enabled: boolean; tools: string[] }
export interface Conversation { id: number; agent_id: number; title: string; created_at: string; updated_at: string }
export interface TutorMessage { id: number; role: "user" | "assistant"; content: string; citations: KnowledgeHit[]; skill_name: string; status: "complete" | "incomplete"; created_at: string; tool_calls?: ToolCall[] }
export type StreamEvent = { event: "meta" | "delta" | "done" | "error"; data: { status?: string; skill?: { id: number; name: string } | null; citations?: KnowledgeHit[]; tool_calls?: ToolCall[]; text?: string; message_id?: number; error?: string } };

export async function readEventStream(body: ReadableStream<Uint8Array>, onEvent: (event: StreamEvent) => void, signal?: AbortSignal) {
  const reader = body.getReader();
  const decoder = new TextDecoder("utf-8", { fatal: true });
  let buffer = "", terminal = false;
  const abort = () => { void reader.cancel(); };
  signal?.addEventListener("abort", abort, { once: true });
  try {
    while (!terminal) {
      if (signal?.aborted) throw new DOMException("已停止生成", "AbortError");
      const { value, done } = await reader.read();
      buffer += done ? decoder.decode() : decoder.decode(value, { stream: true });
      // Normalize CRLF only after each complete line; a trailing CR can span reads.
      let match: RegExpExecArray | null;
      while ((match = /\r?\n\r?\n/.exec(buffer))) {
        const frame = buffer.slice(0, match.index); buffer = buffer.slice(match.index + match[0].length);
        let event = "message"; const data: string[] = [];
        for (const raw of frame.split(/\r?\n/)) {
          if (raw.startsWith("event:")) event = raw.slice(6).trim();
          if (raw.startsWith("data:")) data.push(raw.slice(5).replace(/^ /, ""));
        }
        if (!data.length) continue;
        if (!["meta", "delta", "done", "error"].includes(event)) continue;
        const parsed = { event, data: JSON.parse(data.join("\n")) } as StreamEvent;
        onEvent(parsed);
        if (event === "done" || event === "error") { terminal = true; break; }
      }
      if (buffer.length > 1 << 20) throw new Error("流式帧过大");
      if (done) break;
    }
    if (signal?.aborted) throw new DOMException("已停止生成", "AbortError");
    if (!terminal) throw new Error("回答连接已中断，未收到结束事件");
  } finally { signal?.removeEventListener("abort", abort); await reader.cancel().catch(() => undefined); reader.releaseLock(); }
}

export async function sendTutorMessage(id: number, content: string, onEvent: (event: StreamEvent) => void, signal: AbortSignal) {
  const response = await fetch(`/api/conversations/${id}/messages`, { method: "POST", body: JSON.stringify({ content }), headers: { "Content-Type": "application/json", Authorization: `Bearer ${accessToken}` }, credentials: "omit", signal });
  if (!response.ok) { const error = await response.json().catch(() => ({})) as { error?: string }; throw new ApiError(response.status, error.error || "提问未完成"); }
  if (!response.body || !response.headers.get("Content-Type")?.startsWith("text/event-stream")) throw new Error("服务未返回流式回答");
  return readEventStream(response.body, onEvent, signal);
}

export async function uploadMaterial(
  file: File,
  title: string,
  strategy: ChunkStrategy
): Promise<{ id: number; message: string }> {
  const form = new FormData();
  form.append("file", file);
  if (title.trim()) form.append("title", title.trim());
  form.append("strategy", JSON.stringify(strategy));
  const headers = new Headers();
  if (accessToken) headers.set("Authorization", `Bearer ${accessToken}`);
  const response = await fetch("/api/materials", {
    method: "POST",
    body: form,
    headers,
    credentials: "omit"
  });
  let body: { id?: number; message?: string; error?: string } = {};
  try {
    body = await response.json() as typeof body;
  } catch {
    // The status still carries the failure semantics.
  }
  if (!response.ok || !body.id) throw new ApiError(response.status, body.error || "上传未完成");
  return { id: body.id, message: body.message || "材料已入库" };
}

export async function downloadMaterial(item: Material): Promise<void> {
  const headers = new Headers();
  if (accessToken) headers.set("Authorization", `Bearer ${accessToken}`);
  const response = await fetch(`/api/materials/${item.id}/file`, { credentials: "omit", headers });
  if (!response.ok) {
    let message = response.status === 404 ? "未找到材料" : "下载未完成";
    try {
      const body = (await response.json()) as { error?: string };
      message = body.error || message;
    } catch {
      // Preserve status-based message.
    }
    throw new ApiError(response.status, message);
  }
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = item.original_name;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}

export interface ToolCall {ref: string; status: "ok" | "error" | "denied"; text: string; round: number; call_id: string}
export interface MCPServer {id: number; name: string; base_url: string; has_token: boolean; status: "connected" | "disconnected"; revision: number; protocol_version: string; last_error: string}
export interface MCPTool {name: string; description: string; inputSchema: Record<string, unknown>; hash: string}
export interface MCPChange {id: number; tool_name: string; old: MCPTool | null; new: MCPTool | null; created_at: string}
export interface MCPAudit extends ToolCall {id: number; server_id: number | null; server_name: string; user_id: number; conversation_id: number; question_id: number; duration_ms: number; created_at: string}
export function applyTutorEvent(message: TutorMessage, {event, data}: StreamEvent): TutorMessage {
 if (event === "meta") {
  const calls = new Map((message.tool_calls || []).map(c => [c.call_id, c]));
  for (const call of data.tool_calls || []) calls.set(call.call_id, call);
  return {...message, ...(data.citations !== undefined ? {citations: data.citations} : {}), ...(data.skill !== undefined ? {skill_name: data.skill?.name || ""} : {}), ...(calls.size ? {tool_calls: [...calls.values()]} : {})};
 }
 if (event === "delta") return {...message, content: message.content + (data.text || "")};
 if (event === "done") return {...message, id: data.message_id || message.id, status: "complete"};
 return message;
}

export interface SkillRouting {trigger: string; subject: string; tools: string[]; enabled: boolean}
export interface SkillPreview {format: "course" | "skill-md"; name: string; description: string}

export interface PromptTemplate {id: number; class_id: number; name: string; prompt: string; builtin: boolean}

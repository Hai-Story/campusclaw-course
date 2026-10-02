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

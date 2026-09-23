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

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(path, {
    ...options,
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", ...(options.headers ?? {}) }
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
  login: (username: string, password: string) =>
    request<{ user: User }>("/api/login", {
      method: "POST",
      body: JSON.stringify({ username, password })
    }),
  logout: () => request<void>("/api/logout", { method: "POST" }),
  materials: (query: string) =>
    request<{ materials: Material[] }>(`/api/materials?q=${encodeURIComponent(query)}`),
  material: (id: number) => request<{ material: Material }>(`/api/materials/${id}`)
};

export function uploadMaterial(
  file: File,
  title: string,
  onProgress: (percent: number) => void
): Promise<{ id: number; message: string }> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", "/api/materials");
    xhr.withCredentials = true;
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) onProgress(Math.round((event.loaded / event.total) * 100));
    };
    xhr.onload = () => {
      let body: { id?: number; message?: string; error?: string } = {};
      try {
        body = JSON.parse(xhr.responseText) as typeof body;
      } catch {
        // The status still carries the failure semantics.
      }
      if (xhr.status >= 200 && xhr.status < 300 && body.id) {
        resolve({ id: body.id, message: body.message || "材料已入库" });
      } else {
        reject(new ApiError(xhr.status, body.error || "上传未完成"));
      }
    };
    xhr.onerror = () => reject(new ApiError(0, "网络连接失败"));
    const form = new FormData();
    form.append("file", file);
    if (title.trim()) form.append("title", title.trim());
    xhr.send(form);
  });
}

export async function downloadMaterial(item: Material): Promise<void> {
  const response = await fetch(`/api/materials/${item.id}/file`, { credentials: "same-origin" });
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


import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const user = { id: 1, username: "teacher_a", role: "teacher", class_id: 1, class_name: "A 班" };

describe("Bearer API client", () => {
  let saved: Map<string, string>;
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.resetModules();
    saved = new Map();
    fetchMock = vi.fn();
    vi.stubGlobal("sessionStorage", {
      getItem: (key: string) => saved.get(key) ?? null,
      setItem: (key: string, value: string) => saved.set(key, value),
      removeItem: (key: string) => saved.delete(key)
    });
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("logs in without credentials and sends Bearer on later requests", async () => {
    saved.set("campus-access-token-v2", "stale-token");
    fetchMock
      .mockResolvedValueOnce(new Response(JSON.stringify({ user, token: "new-token" }), { status: 200 }))
      .mockResolvedValueOnce(new Response(JSON.stringify({ materials: [] }), { status: 200 }));
    const { api } = await import("./api");

    await api.login("teacher_a", "password");
    await api.materials("");

    const loginOptions = fetchMock.mock.calls[0][1] as RequestInit;
    expect(loginOptions.credentials).toBe("omit");
    expect((loginOptions.headers as Headers).has("Authorization")).toBe(false);
    const materialsOptions = fetchMock.mock.calls[1][1] as RequestInit;
    expect(materialsOptions.credentials).toBe("omit");
    expect((materialsOptions.headers as Headers).get("Authorization")).toBe("Bearer new-token");
    expect(saved.get("campus-access-token-v2")).toBe("new-token");
  });

  it("validates a pasted token before storing it and clears it on logout", async () => {
    fetchMock
      .mockResolvedValueOnce(new Response(JSON.stringify({ user }), { status: 200 }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    const { api } = await import("./api");

    await api.useToken("Bearer pasted-token");
    expect((fetchMock.mock.calls[0][1].headers as Headers).get("Authorization")).toBe("Bearer pasted-token");
    expect(saved.get("campus-access-token-v2")).toBe("pasted-token");
    await api.logout();
    expect((fetchMock.mock.calls[1][1].headers as Headers).get("Authorization")).toBe("Bearer pasted-token");
    expect(saved.has("campus-access-token-v2")).toBe(false);
  });

  it("uses credential-free multipart upload with a Bearer header", async () => {
    saved.set("campus-access-token-v2", "upload-token");
    fetchMock.mockResolvedValue(new Response(JSON.stringify({ id: 4, message: "ok" }), { status: 201 }));
    const { uploadMaterial } = await import("./api");

    await uploadMaterial(new File(["# note"], "note.md", { type: "text/markdown" }), "note", { mode: "auto" });

    const options = fetchMock.mock.calls[0][1] as RequestInit;
    expect(options.credentials).toBe("omit");
    expect((options.headers as Headers).get("Authorization")).toBe("Bearer upload-token");
    expect(options.body).toBeInstanceOf(FormData);
    expect((options.headers as Headers).has("Content-Type")).toBe(false);
  });
});
